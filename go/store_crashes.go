package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TODO: use an int for ProgramName and ProgramVersion and ip addr
// to use less memory and make linear scan faster
type Crash struct {
	Id             int
	CreatedOn      time.Time
	ProgramName    string
	ProgramVersion string
	IpAddrInternal string
	Sha1           [20]byte
}

type StoreCrashes struct {
	sync.Mutex
	dataDir  string
	crashes  []Crash
	dataFile *os.File
}

func (c *Crash) IpAddress() string {
	return ipAddrInternalToOriginal(c.IpAddrInternal)
}

func ipAddrInternalToOriginal(s string) string {
	// check if ipv4 in hex form
	if len(s) == 8 {
		if d, err := hex.DecodeString(s); err != nil {
			return s
		} else {
			return fmt.Sprintf("%d.%d.%d.%d", d[0], d[1], d[2], d[3])
		}
	}
	// other format (ipv6?)
	return s
}

func ipAddrToInternal(ipAddr string) string {
	var nums [4]uint32
	parts := strings.Split(ipAddr, ".")
	if len(parts) == 4 {
		for n, p := range parts {
			num, _ := strconv.Atoi(p)
			nums[n] = uint32(num)
		}
		n := (nums[0] << 24) | (nums[1] << 16) + (nums[2] << 8) | nums[3]
		return fmt.Sprintf("%x", n)
	}
	// I assume it's ipv6
	return ipAddr
}

// parse:
// C/vs1mJI02u0HBsHPceGfxy/Q+JE|1351741403|SumatraPDF|2.1.1|6e8e602f
func (s *StoreCrashes) parseCrash(line []byte) {
	parts := strings.Split(string(line[1:]), "|")
	if len(parts) != 5 {
		panic("len(parts) != 5")
	}
	msgSha1b64 := parts[0] + "="
	createdOnSecondsStr := parts[1]
	programName := parts[2]
	programVersion := parts[3]
	ipAddrInternal := parts[4]

	createdOnSeconds, err := strconv.Atoi(createdOnSecondsStr)
	if err != nil {
		panic("createdOnSeconds not a number")
	}
	createdOn := time.Unix(int64(createdOnSeconds), 0)

	msgSha1, err := base64.StdEncoding.DecodeString(msgSha1b64)
	if err != nil {
		panic("msgSha1b64 not valid base64")
	}
	if len(msgSha1) != 20 {
		panic("len(msgSha1) != 20")
	}

	c := Crash{
		Id:             len(s.crashes),
		CreatedOn:      createdOn,
		ProgramName:    programName,
		ProgramVersion: programVersion,
		IpAddrInternal: ipAddrInternal,
	}
	copy(c.Sha1[:], msgSha1)
	if !s.MessageFileExists(c.Sha1[:]) {
		panic("message file doesn't exist")
	}
	s.crashes = append(s.crashes, c)
}

func (s *StoreCrashes) readExistingCrashesData(fileDataPath string) error {
	d, err := ReadFileAll(fileDataPath)
	if err != nil {
		return err
	}

	for len(d) > 0 {
		idx := bytes.IndexByte(d, '\n')
		if -1 == idx {
			// TODO: this could happen if the last record was only
			// partially written. Should I just ignore it?
			panic("idx shouldn't be -1")
		}
		line := d[:idx]
		d = d[idx+1:]
		c := line[0]
		if c == 'C' {
			s.parseCrash(line)
		} else {
			fmt.Printf("'%s'\n", string(line))
			panic("Unexpected line type")
		}
	}
	return nil
}

func NewStoreCrashes(dataDir string) (*StoreCrashes, error) {
	dataFilePath := filepath.Join(dataDir, "crashesdata.txt")
	store := &StoreCrashes{
		dataDir: dataDir,
		crashes: make([]Crash, 0),
	}
	var err error
	if PathExists(dataFilePath) {
		err = store.readExistingCrashesData(dataFilePath)
		if err != nil {
			logger.Errorf("NewStoreCrashes(): readExistingCrashesData() failed with %s\n", err.Error())
			return nil, err
		}
	} else {
		f, err := os.Create(dataFilePath)
		if err != nil {
			logger.Errorf("NewStoreCrashes(): os.Create(%s) failed with %s", dataFilePath, err.Error())
			return nil, err
		}
		f.Close()
	}
	store.dataFile, err = os.OpenFile(dataFilePath, os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		logger.Errorf("NewStoreCrashes(): os.OpenFile(%s) failed with %s", dataFilePath, err.Error())
		return nil, err
	}
	logger.Noticef("crashes: %d", len(store.crashes))
	return store, nil
}

func (s *StoreCrashes) CrashesCount() int {
	s.Lock()
	defer s.Unlock()
	return len(s.crashes)
}

func blobCrahesPath(dir, sha1 string) string {
	d1 := sha1[:2]
	d2 := sha1[2:4]
	return filepath.Join(dir, "blobs_crashes", d1, d2, sha1)
}

func (s *StoreCrashes) MessageFilePath(sha1 []byte) string {
	sha1Str := hex.EncodeToString(sha1)
	return blobCrahesPath(s.dataDir, sha1Str)
}

func (s *StoreCrashes) MessageFileExists(sha1 []byte) bool {
	p := s.MessageFilePath(sha1)
	return PathExists(p)
}

func (s *StoreCrashes) appendString(str string) error {
	_, err := s.dataFile.WriteString(str)
	if err != nil {
		logger.Errorf("StoreCrashes.appendString() error: %s\n", err.Error())
	}
	return err
}

func (s *StoreCrashes) writeMessageAsSha1(msg []byte, sha1 []byte) error {
	path := s.MessageFilePath(sha1)
	err := WriteBytesToFile(msg, path)
	if err != nil {
		logger.Errorf("StoreCrashes.writeMessageAsSha1(): failed to write %s with error %s", path, err.Error())
	}
	return err
}

func (s *StoreCrashes) newCrashId() int {
	return len(s.crashes)
}

func ip2str(s string) uint32 {
	var nums [4]uint32
	parts := strings.Split(s, ".")
	for n, p := range parts {
		num, _ := strconv.Atoi(p)
		nums[n] = uint32(num)
	}
	return (nums[0] << 24) | (nums[1] << 16) + (nums[2] << 8) | nums[3]
}

func serCrash(c *Crash) string {
	s1 := base64.StdEncoding.EncodeToString(c.Sha1[:])
	s1 = s1[:len(s1)-1] // remove '=' from the end
	s2 := fmt.Sprintf("%d", c.CreatedOn.Unix())
	s3 := remSep(c.ProgramName)
	s4 := remSep(c.ProgramVersion)
	s5 := c.IpAddrInternal
	return fmt.Sprintf("C%s|%s|%s|%s|%s\n", s1, s2, s3, s4, s5)
}