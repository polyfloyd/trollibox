package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"

	"trollibox/src/handler/api"
	"trollibox/src/handler/webui"
	"trollibox/src/jukebox"
	"trollibox/src/util"
)

type ColorConfig struct {
	Background     string `yaml:"background"`
	BackgroundElem string `yaml:"background_elem"`
	Text           string `yaml:"text"`
	TextInactive   string `yaml:"text_inactive"`
	Accent         string `yaml:"accent"`
}

type webUI struct {
	build, version string
	colorConfig    ColorConfig
	urlRoot        string
	jukebox        *jukebox.Jukebox
}

func New(build, version string, colorConfig ColorConfig, urlRoot string, jukebox *jukebox.Jukebox) chi.Router {
	web := webUI{
		build:       build,
		version:     version,
		colorConfig: colorConfig,
		urlRoot:     urlRoot,
		jukebox:     jukebox,
	}

	service := chi.NewRouter()
	service.Use(util.LogHandler)
	service.Use(middleware.Compress(5))

	service.Mount("/static", http.StripPrefix("/static/", http.FileServer(http.FS(web.fs()))))
	service.Get("/static/default-album-art.svg", web.defaultAlbumArt())

	service.Get("/", web.redirectToDefaultPlayer)
	service.Get("/player/{player}", web.browserPage)
	service.Get("/player/{player}/{view}", web.browserPage)
	service.Route("/data", func(r chi.Router) {
		api.InitRouter(r, web.jukebox)
	})

	return service
}

func (web *webUI) fs() fs.FS {
	return webui.Files(web.build)
}

func (web *webUI) baseParamMap() map[string]interface{} {
	playerNames, _ := web.jukebox.Players(context.TODO())
	return map[string]interface{}{
		"urlroot": web.urlRoot,
		"build":   web.build,
		"version": web.version,
		"time":    time.Now(),
		"players": playerNames,
		"colors": map[string]string{
			"bg":           web.colorConfig.Background,
			"bgElem":       web.colorConfig.BackgroundElem,
			"text":         web.colorConfig.Text,
			"textInactive": web.colorConfig.TextInactive,
			"accent":       web.colorConfig.Accent,
		},
	}
}

var pageTemplate *template.Template

func (web *webUI) mkTemplate() *template.Template {
	b, err := fs.ReadFile(web.fs(), "index.html")
	if err != nil {
		panic(err)
	}
	return template.Must(template.New("page").Parse(string(b)))
}

func (web *webUI) getTemplate() *template.Template {
	if web.build == "debug" {
		return web.mkTemplate()
	}
	if pageTemplate == nil {
		pageTemplate = web.mkTemplate()
	}
	return pageTemplate
}

func (web *webUI) browserPage(w http.ResponseWriter, r *http.Request) {
	params := web.baseParamMap()
	params["player"] = chi.URLParam(r, "player")
	switch view := r.FormValue("view"); view {
	case "", "search", "albums", "genres", "files", "streams", "queuer", "player":
		params["view"] = view
	default:
		web.redirectToDefaultPlayer(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if err := web.getTemplate().Execute(w, params); err != nil {
		log.Fatal(err)
	}
}

func (web *webUI) redirectToDefaultPlayer(w http.ResponseWriter, r *http.Request) {
	defaultPlayer, err := web.jukebox.DefaultPlayer(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "could not find a player to redirect to: %v", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/player/%s", defaultPlayer), http.StatusTemporaryRedirect)
}

func (web *webUI) defaultAlbumArt() http.HandlerFunc {
	filename := "default-album-art.svg"
	svg, err := fs.ReadFile(web.fs(), filename)
	if err != nil {
		panic(err)
	}
	recolored := bytes.Replace(svg, []byte("#ffffff"), []byte(web.colorConfig.Accent), -1)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		info, _ := fs.Stat(web.fs(), filename)
		http.ServeContent(w, req, filename, info.ModTime(), bytes.NewReader(recolored))
	})
}
