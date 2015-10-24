package main

import (
	"html/template"
	"io"
	"time"

	assets "./assets-go"
)

const PAGE_BASE = "view/page.html"

var (
	pageTemplates  = map[string]*template.Template{}
	httpCacheSince = time.Now()
)

func RenderPage(name string, wr io.Writer, data interface{}) error {
	return getPageTemplate(name).ExecuteTemplate(wr, "page", data)
}

func getPageTemplate(name string) *template.Template {
	if page, ok := pageTemplates[name]; !ok || BUILD == "debug" {
		base := template.Must(template.New(PAGE_BASE).Parse(string(assets.MustAsset(PAGE_BASE))))
		page = template.Must(base.New(name).Parse(string(assets.MustAsset(name))))
		pageTemplates[name] = page
		return page
	} else {
		return page
	}
}

func HttpCacheTime() time.Time {
	if BUILD == "debug" {
		return time.Now()
	} else {
		return httpCacheSince
	}
}
