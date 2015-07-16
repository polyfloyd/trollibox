package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/gorilla/mux"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"time"
	assets "./assets-go"
)

const (
	PUBLIC   = "public"
	CONFFILE = "config.json"
)

var (
	BUILD   = strings.Trim(string(assets.MustAsset("_BUILD")), "\n ")
	VERSION = strings.Trim(string(assets.MustAsset("_VERSION")), "\n ")
)

var (
	config Config
	static map[string][]string

	httpCacheSince    = time.Now()
	httpCacheSinceStr = httpCacheSince.Format(http.TimeFormat)
	httpCacheUntil    = httpCacheSince.AddDate(1, 0, 0) // 1 year
	httpCacheUntilStr = httpCacheUntil.Format(http.TimeFormat)
)

type Config struct {
	Address string `json:"listen-address"`
	URLRoot string `json:"url-root"`

	StorageDir string `json:"storage-dir"`

	Mpd struct {
		Host     string  `json:"host"`
		Port     int     `json:"port"`
		Password *string `json:"password"`
	} `json:"mpd"`
}


type AssetServeHandler struct {
	name string
}

func (h *AssetServeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(h.name)))
	io.Copy(w, bytes.NewReader(assets.MustAsset(h.name)))
}


func main() {
	log.Printf("Version: %v (%v)\n", VERSION, BUILD)

	// Prevent blocking routines to lock up the whole program
	runtime.GOMAXPROCS(8)

	configFile := flag.String("conf", CONFFILE, "Path to the configuration file")
	flag.Parse()

	if in, err := os.Open(*configFile); err != nil {
		log.Fatal(err)
	} else if err := json.NewDecoder(in).Decode(&config); err != nil {
		log.Fatal(err)
	}

	if err := SetStorageDir(config.StorageDir); err != nil {
		log.Fatal(err)
	}
	log.Printf("Using \"%v\" for storage", config.StorageDir)
	if err := InitStreams(); err != nil {
		log.Fatal(err)
	}

	queuer, err := NewQueuer("queuer")
	if err  != nil {
		log.Fatal(err)
	}

	player, err := NewPlayer(config.Mpd.Host, config.Mpd.Port, config.Mpd.Password, queuer)
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()
	rCached := mux.NewRouter()

	static = getStaticAssets(assets.AssetNames())
	for _, file := range assets.AssetNames() {
		if !strings.HasPrefix(file, PUBLIC) {
			continue
		}
		urlPath := strings.TrimPrefix(file, PUBLIC)
		rCached.Path(urlPath).Handler(&AssetServeHandler{name: file})
	}

	rCached.Path("/").HandlerFunc(htBrowserPage())
	rCached.Path("/player").HandlerFunc(htPlayerPage())
	rCached.Path("/queuer").HandlerFunc(htQueuerPage())
	htDataAttach(r.PathPrefix("/data/").Subrouter(), player)

	// 404
	rCached.NewRoute().HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		http.Redirect(res, req, "/", http.StatusTemporaryRedirect)
	})

	if BUILD == "release" {
		r.NewRoute().HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mod := req.Header.Get("If-Modified-Since")
			if mod != "" {
				modTime, err := time.Parse(http.TimeFormat, mod)
				if err == nil && modTime.After(httpCacheSince) && modTime.Before(httpCacheUntil) {
					w.WriteHeader(304)
					return
				}
			}

			w.Header().Set("Cache-Control", "public, max-age=290304000")
			w.Header().Set("Last-Modified", httpCacheSinceStr)
			w.Header().Set("Expires", httpCacheUntilStr)
			rCached.ServeHTTP(w, req)
		})
	} else {
		r.NewRoute().Handler(rCached)
	}

	log.Printf("Now accepting HTTP connections on %v", config.Address)
	server := &http.Server{
		Addr:           config.Address,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(server.ListenAndServe())
}

func getStaticAssets(files []string) (static map[string][]string) {
	static = map[string][]string{
		"js":  []string{},
		"css": []string{},
	}

	for _, file := range files {
		if !strings.HasPrefix(file, PUBLIC) {
			continue
		}
		urlPath := strings.TrimPrefix(file, PUBLIC+"/")

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

	return
}

func GetBaseParamMap() map[string]interface{} {
	return map[string]interface{}{
		"urlroot":  config.URLRoot,
		"version":  VERSION,
		"assets":   static,
		"time":     time.Now(),
	}
}
