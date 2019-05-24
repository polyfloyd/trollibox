package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"

	"github.com/polyfloyd/trollibox/src/api"
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

func htBrowserPage(config *config, players player.List) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		params := baseParamMap(config, players)
		params["player"] = chi.URLParam(r, "player")

		w.Header().Set("Content-Type", "text/html")
		if err := getTemplate().Execute(w, params); err != nil {
			log.Fatal(err)
		}
	}
}

func htRedirectToDefaultPlayer(config *config, players player.List) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defaultPlayer := ""
		if pl, err := players.PlayerByName(config.DefaultPlayer); err == nil && pl != nil {
			defaultPlayer = config.DefaultPlayer
		} else if names, err := players.PlayerNames(); err == nil && len(names) > 0 {
			defaultPlayer = names[0]
		} else {
			api.WriteError(w, r, fmt.Errorf("error finding a player to redirect to: %v", err))
			return
		}
		http.Redirect(w, r, "/player/"+defaultPlayer, http.StatusTemporaryRedirect)
	}
}
