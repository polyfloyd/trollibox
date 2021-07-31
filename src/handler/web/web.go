package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/polyfloyd/trollibox/src/handler/api"
	"github.com/polyfloyd/trollibox/src/handler/webui"
	"github.com/polyfloyd/trollibox/src/jukebox"
	"github.com/polyfloyd/trollibox/src/util"
)

const publicDir = "public"

var static = getStaticAssets(webui.AssetNames())

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
	for _, file := range webui.AssetNames() {
		if !strings.HasPrefix(file, publicDir) {
			continue
		}
		urlPath := strings.TrimPrefix(file, publicDir)
		service.Get(urlPath, assetServeHandler(file).ServeHTTP)
	}
	service.Get("/img/default-album-art.svg", web.defaultAlbumArt())

	service.Get("/", web.redirectToDefaultPlayer)
	service.Get("/player/{player}", web.browserPage)
	service.Route("/data", func(r chi.Router) {
		api.InitRouter(r, web.jukebox)
	})

	return service
}

func getStaticAssets(files []string) map[string][]string {
	static := map[string][]string{
		"js":  {},
		"css": {},
	}
	for _, file := range files {
		if !strings.HasPrefix(file, publicDir) {
			continue
		}
		urlPath := strings.TrimPrefix(file, publicDir+"/")
		switch path.Ext(file) {
		case ".css":
			static["css"] = append(static["css"], urlPath)
		case ".js":
			static["js"] = append(static["js"], urlPath)
		}
	}
	for _, a := range static {
		sort.Strings(a)
	}
	return static
}

func (web *webUI) baseParamMap() map[string]interface{} {
	playerNames, _ := web.jukebox.Players(context.TODO())
	return map[string]interface{}{
		"urlroot": web.urlRoot,
		"version": web.version,
		"assets":  static,
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

var pageTemplate = mkTemplate()

func mkTemplate() *template.Template {
	return template.Must(template.New("page").Parse(string(webui.MustAsset("view/page.html"))))
}

func (web *webUI) getTemplate() *template.Template {
	if web.build == "debug" {
		return mkTemplate()
	}
	return pageTemplate
}

func (web *webUI) browserPage(w http.ResponseWriter, r *http.Request) {
	params := web.baseParamMap()
	params["player"] = chi.URLParam(r, "player")

	w.Header().Set("Content-Type", "text/html")
	if err := web.getTemplate().Execute(w, params); err != nil {
		log.Fatal(err)
	}
}

func (web *webUI) redirectToDefaultPlayer(w http.ResponseWriter, r *http.Request) {
	defaultPlayer, err := web.jukebox.DefaultPlayer(r.Context())
	if err != nil {
		api.WriteError(w, r, fmt.Errorf("error finding a player to redirect to: %v", err))
		return
	}
	http.Redirect(w, r, "/player/"+defaultPlayer, http.StatusTemporaryRedirect)
}

type assetServeHandler string

func (h assetServeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := string(h)
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	info, err := webui.AssetInfo(name)
	if err != nil {
		log.Errorf("Could not serve %q: %v", name, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, req, name, info.ModTime(), bytes.NewReader(webui.MustAsset(name)))
}

func (web *webUI) defaultAlbumArt() http.HandlerFunc {
	filename := "default-album-art.svg"
	recolored := bytes.Replace(webui.MustAsset(filename), []byte("#ffffff"), []byte(web.colorConfig.Accent), -1)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		info, _ := webui.AssetInfo(filename)
		http.ServeContent(w, req, filename, info.ModTime(), bytes.NewReader(recolored))
	})
}
