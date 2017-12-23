package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/polyfloyd/trollibox/src/assets"
	"github.com/polyfloyd/trollibox/src/player"
)

var pageTemplate = mkTemplate()

func mkTemplate() *template.Template {
	return template.Must(template.New("page").Parse(string(assets.MustAsset("view/page.html"))))
}

func getTemplate() *template.Template {
	if build == "debug" {
		return mkTemplate()
	}
	return pageTemplate
}

func htBrowserPage(config *config, players player.List) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		params := baseParamMap(config, players)
		params["player"] = mux.Vars(req)["player"]

		res.Header().Set("Content-Type", "text/html")
		if err := getTemplate().Execute(res, params); err != nil {
			panic(err)
		}
	}
}

func htRedirectToDefaultPlayer(config *config, players player.List) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		defaultPlayer := ""
		if pl, err := players.PlayerByName(config.DefaultPlayer); err == nil && pl != nil {
			defaultPlayer = config.DefaultPlayer
		} else if names, err := players.PlayerNames(); err == nil && len(names) > 0 {
			defaultPlayer = names[0]
		} else {
			writeError(req, res, fmt.Errorf("error finding a player to redirect to: %v", err))
			return
		}
		http.Redirect(res, req, "/player/"+defaultPlayer, http.StatusTemporaryRedirect)
	}
}

func htJSONContent(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
}
