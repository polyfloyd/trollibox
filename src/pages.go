package main

import (
	"html/template"
	"net/http"

	assets "./assets-go"
	"./player"
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

func htBrowserPage(config *Config, players map[string]player.Player, playerName string) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		params := baseParamMap(config, players)
		params["player"] = playerName

		res.Header().Set("Content-Type", "text/html")
		if err := getTemplate().Execute(res, params); err != nil {
			panic(err)
		}
	}
}
