package main

import (
	"html/template"
	"net/http"

	assets "./assets-go"
	"github.com/gorilla/mux"
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

func htBrowserPage(config *Config, players PlayerList) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		params := baseParamMap(config, players)
		params["player"] = mux.Vars(req)["player"]

		res.Header().Set("Content-Type", "text/html")
		if err := getTemplate().Execute(res, params); err != nil {
			panic(err)
		}
	}
}

func htRedirectToDefaultPlayer(config *Config, players PlayerList) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		defaultPlayer := ""
		if pl := players.ActivePlayerByName(config.DefaultPlayer); pl != nil {
			defaultPlayer = config.DefaultPlayer
		} else if names := players.ActivePlayers(); len(names) > 0 {
			defaultPlayer = names[0]
		}
		http.Redirect(res, req, "/player/"+defaultPlayer, http.StatusTemporaryRedirect)
	}
}

func hmJsonContent(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
}
