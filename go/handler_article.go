package main

import (
	"html/template"
	"net/http"
	"strings"
)

type DisplayArticle struct {
	*Article
	HtmlBody template.HTML
}

func (a *DisplayArticle) PublishedOnShort() string {
	return a.PublishedOn.Format("Jan 2 2006")
}

// /article/*, /blog/*, /kb/*
func handleArticle(w http.ResponseWriter, r *http.Request) {
	if redirectIfNeeded(w, r) {
		return
	}

	// /blog/ and /kb/ are only for redirects, we only handle /article/
	// at this point
	url := r.URL.Path
	if !strings.HasPrefix(url, "/article/") {
		serve404(w, r)
		return
	}
	isAdmin := IsAdmin(r)

	// we expect /article/$shortId/$url
	parts := strings.SplitN(url[len("/article/"):], "/", 2)
	if len(parts) != 2 {
		serve404(w, r)
		return
	}

	articleId := UnshortenId(parts[0])
	prev, article, next, pos := getCachedArticlesById(articleId, isAdmin)
	if nil == article {
		serve404(w, r)
		return
	}

	displayArticle := &DisplayArticle{Article: article}

	ver := article.CurrVersion()
	msgHtml := articleBodyCache.GetHtml(ver.Sha1, ver.Format)
	displayArticle.HtmlBody = template.HTML(msgHtml)

	model := struct {
		IsAdmin        bool
		AnalyticsCode  string
		JqueryUrl      string
		PrettifyJsUrl  string
		PrettifyCssUrl string
		PageTitle      string
		Article        *DisplayArticle
		NextArticle    *Article
		PrevArticle    *Article
		LogInOutUrl    string
		ArticlesJsUrl  string
		TagsDisplay    string
		ArticleNo      int
		ArticlesCount  int
	}{
		IsAdmin:        isAdmin,
		AnalyticsCode:  *config.AnalyticsCode,
		JqueryUrl:      jQueryUrl(),
		PrettifyJsUrl:  prettifyJsUrl(),
		PrettifyCssUrl: prettifyCssUrl(),
		LogInOutUrl:    getLogInOutUrl(r),
		Article:        displayArticle,
		NextArticle:    next,
		PrevArticle:    prev,
		PageTitle:      article.Title,
		ArticlesCount:  store.ArticlesCount(),
		ArticleNo:      pos + 1,
		ArticlesJsUrl:  getArticlesJsUrl(isAdmin),
	}

	ExecTemplate(w, tmplArticle, model)
}
