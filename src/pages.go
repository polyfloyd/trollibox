package main

import (
	"html/template"
	"net/http"

	assets "./assets-go"
)

var pageTemplate = mkTemplate()

func mkTemplate() *template.Template {
	return template.Must(template.New("page").Parse(string(assets.MustAsset("view/page.html"))))
}

func getTemplate() *template.Template {
	if BUILD == "debug" {
		return mkTemplate()
	}
	return pageTemplate
}

func htBrowserPage() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		params := GetBaseParamMap()

		if err := getTemplate().Execute(res, params); err != nil {
			panic(err)
		}
	}
}
