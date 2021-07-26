package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"github.com/polyfloyd/trollibox/src/api"
	"github.com/polyfloyd/trollibox/src/assets"
	"github.com/polyfloyd/trollibox/src/jukebox"
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

func htRedirectToDefaultPlayer(jukebox *jukebox.Jukebox) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defaultPlayer, err := jukebox.DefaultPlayer(r.Context())
		if err != nil {
			api.WriteError(w, r, fmt.Errorf("error finding a player to redirect to: %v", err))
			return
		}
		http.Redirect(w, r, "/player/"+defaultPlayer, http.StatusTemporaryRedirect)
	}
}
