package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	assets "./assets-go"
	"./player"
	"./player/mpd"
	"./player/slimserver"
	"./stream"
	"./util"
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
	config  Config
	static  map[string][]string
	players = map[string]player.Player{}
)

type Config struct {
	Address string `json:"listen-address"`
	URLRoot string `json:"url-root"`

	StorageDir string `json:"storage-dir"`

	Mpd []struct {
		Name     string  `json:"name"`
		Host     string  `json:"host"`
		Port     int     `json:"port"`
		Password *string `json:"password"`
	} `json:"mpd"`

	SlimServer *struct {
		Host     string  `json:"host"`
		Port     int     `json:"port"`
		Username *string `json:"username"`
		Password *string `json:"password"`
		WebUrl   string  `json:"weburl"`
	} `json:"slimserver"`
}

func (conf *Config) Load(filename string) error {
	if in, err := os.Open(filename); err != nil {
		return fmt.Errorf("Could not open config file: %v", err)
	} else if err := json.NewDecoder(in).Decode(conf); err != nil {
		return fmt.Errorf("Unable to decode config: %v", err)
	}
	return nil
}

type AssetServeHandler string

func (h AssetServeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := string(h)
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	info, _ := assets.AssetInfo(name)
	http.ServeContent(w, req, name, info.ModTime(), bytes.NewReader(assets.MustAsset(name)))
}

func main() {
	log.Printf("Version: %v (%v)\n", VERSION, BUILD)

	// Prevent blocking routines to lock up the whole program
	runtime.GOMAXPROCS(8)

	configFile := flag.String("conf", CONFFILE, "Path to the configuration file")
	flag.Parse()

	if err := config.Load(*configFile); err != nil {
		log.Fatal(err)
	}

	storeDir := strings.Replace(config.StorageDir, "~", os.Getenv("HOME"), 1)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		log.Fatalf("Unable to create config dir: %v", err)
	}
	log.Printf("Using \"%s\" for storage", storeDir)

	streamdb, err := stream.NewDB(path.Join(storeDir, "streams.json"))
	if err != nil {
		log.Fatalf("Unable to create stream database: %v", err)
	}
	queuer, err := player.NewQueuer(path.Join(storeDir, "queuer.json"))
	if err != nil {
		log.Fatalf("Unable to create queuer: %v", err)
	}

	addPlayer := func(pl player.Player, name string) error {
		if match, _ := regexp.MatchString("^\\w+$", name); !match {
			return fmt.Errorf("Invalid player name: %q", name)
		}
		if _, ok := players[name]; ok {
			return fmt.Errorf("Duplicate player name: %q", name)
		}
		players[name] = pl
		return nil
	}

	for _, mpdConf := range config.Mpd {
		mpdPlayer, err := mpd.NewPlayer(mpdConf.Host, mpdConf.Port, mpdConf.Password)
		if err != nil {
			log.Fatalf("Unable to create MPD player: %v", err)
		}
		if err := addPlayer(mpdPlayer, mpdConf.Name); err != nil {
			log.Fatal(err)
		}
	}
	if config.SlimServer != nil {
		slimServ, err := slimserver.Connect(
			config.SlimServer.Host,
			config.SlimServer.Port,
			config.SlimServer.Username,
			config.SlimServer.Password,
			config.SlimServer.WebUrl,
		)
		if err != nil {
			log.Fatalf("Unable to connect to SlimServer: %v", err)
		}
		players, err := slimServ.Players()
		if err != nil {
			log.Fatal(err)
		}

		for _, pl := range players {
			if err := addPlayer(pl, pl.Name); err != nil {
				log.Fatal(err)
			}
		}
	}

	if len(players) == 0 {
		log.Fatal("No players configured")
	}
	var defaultPlayer string
	for name := range players {
		defaultPlayer = name
		break
	}

	for name, pl := range players {
		go func(pl player.Player, name string) {
			for {
				log.Printf("Error while autoqueueing for %s: %v", name, player.AutoQueue(queuer, pl))
			}
		}(pl, name)
	}

	r := mux.NewRouter()
	r.Handle("/", http.RedirectHandler("/player/"+defaultPlayer, http.StatusTemporaryRedirect))
	for name, pl := range players {
		func(name string, pl player.Player) {
			playerOnline := func(r *http.Request, rm *mux.RouteMatch) bool {
				return pl.Available()
			}
			r.Path(fmt.Sprintf("/player/%s", name)).MatcherFunc(playerOnline).HandlerFunc(htBrowserPage(name))
			htPlayerDataAttach(r.PathPrefix(fmt.Sprintf("/data/player/%s/", name)).MatcherFunc(playerOnline).Subrouter(), pl, streamdb)
		}(name, pl)
	}

	static = getStaticAssets(assets.AssetNames())
	for _, file := range assets.AssetNames() {
		if !strings.HasPrefix(file, PUBLIC) {
			continue
		}
		urlPath := strings.TrimPrefix(file, PUBLIC)
		r.Path(urlPath).Handler(AssetServeHandler(file))
	}
	htDataAttach(r.PathPrefix("/data/").Subrouter(), queuer, streamdb)

	log.Printf("Now accepting HTTP connections on %v", config.Address)
	server := &http.Server{
		Addr:           config.Address,
		Handler:        util.Gzip(r),
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
	playerNames := make([]string, 0, len(players))
	for name := range players {
		playerNames = append(playerNames, name)
	}
	sort.Strings(playerNames)
	return map[string]interface{}{
		"urlroot": config.URLRoot,
		"version": VERSION,
		"assets":  static,
		"time":    time.Now(),
		"players": playerNames,
	}
}
