package main

import (
	"bytes"
	"encoding/json"
	"flag"
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
	"./player"
	"./player/mpd"
	"github.com/gorilla/mux"
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
	http.ServeContent(w, req, h.name, HttpCacheTime(), bytes.NewReader(assets.MustAsset(h.name)))
}

func main() {
	log.Printf("Version: %v (%v)\n", VERSION, BUILD)

	// Prevent blocking routines to lock up the whole program
	runtime.GOMAXPROCS(8)

	configFile := flag.String("conf", CONFFILE, "Path to the configuration file")
	flag.Parse()

	if in, err := os.Open(*configFile); err != nil {
		log.Fatalf("Could not open config file: %v", err)
	} else if err := json.NewDecoder(in).Decode(&config); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	storeDir := strings.Replace(config.StorageDir, "~", os.Getenv("HOME"), 1)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		log.Fatalf("Unable to create config dir: %v", err)
	}
	log.Printf("Using \"%s\" for storage", storeDir)

	streamdb, err := player.NewStreamDB(path.Join(storeDir, "streams.json"))
	if err != nil {
		log.Fatalf("Unable to create stream database: %v", err)
	}
	queuer, err := player.NewQueuer(path.Join(storeDir, "queuer.json"))
	if err != nil {
		log.Fatalf("Unable to create queuer: %v", err)
	}
	mpdPlayer, err := mpd.NewPlayer(config.Mpd.Host, config.Mpd.Port, config.Mpd.Password)
	if err != nil {
		log.Fatalf("Unable to create MPD player: %v", err)
	}

	go func() {
		for {
			log.Printf("Error while autoqueueing: %v", player.AutoQueue(queuer, mpdPlayer))
		}
	}()

	r := mux.NewRouter()

	static = getStaticAssets(assets.AssetNames())
	for _, file := range assets.AssetNames() {
		if !strings.HasPrefix(file, PUBLIC) {
			continue
		}
		urlPath := strings.TrimPrefix(file, PUBLIC)
		r.Path(urlPath).Handler(&AssetServeHandler{name: file})
	}

	r.Path("/").HandlerFunc(htBrowserPage())
	htDataAttach(r.PathPrefix("/data/").Subrouter(), mpdPlayer, queuer, streamdb)

	log.Printf("Now accepting HTTP connections on %v", config.Address)
	server := &http.Server{
		Addr:           config.Address,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatalf("Unable to start webserver: %v", server.ListenAndServe())
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
		"urlroot": config.URLRoot,
		"version": VERSION,
		"assets":  static,
		"time":    time.Now(),
	}
}
