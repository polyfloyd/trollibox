package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	assets "./assets-go"
	"./filter"
	"./filter/keyed"
	"./filter/ruled"
	"./player"
	"./player/mpd"
	"./player/slimserver"
	"./stream"
	"./util"
	"github.com/gorilla/mux"
)

const (
	PUBLIC_DIR = "public"
	CONFFILE   = "config.json"
)

var (
	BUILD   = strings.Trim(string(assets.MustAsset("_BUILD")), "\n ")
	VERSION = strings.Trim(string(assets.MustAsset("_VERSION")), "\n ")
)

var static = getStaticAssets(assets.AssetNames())

type Config struct {
	Address string `json:"listen-address"`
	URLRoot string `json:"url-root"`

	StorageDir string `json:"storage-dir"`

	AutoQueue     bool   `json:"autoqueue"`
	DefaultPlayer string `json:"default-player"`

	Mpd []struct {
		Name     string  `json:"name"`
		Network  string  `json:"network"`
		Address  string  `json:"address"`
		Password *string `json:"password"`
	} `json:"mpd"`

	SlimServer *struct {
		Network  string  `json:"network"`
		Address  string  `json:"address"`
		Username *string `json:"username"`
		Password *string `json:"password"`
		WebUrl   string  `json:"weburl"`
	} `json:"slimserver"`
}

func (conf *Config) Load(filename string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Unable to decode config: %v", err)
	}
	multilineCommentRe := regexp.MustCompile("(?s)/\\*.*\\*/")
	content = multilineCommentRe.ReplaceAll(content, []byte{})
	singlelineCommentRe := regexp.MustCompile("(\"[^\"]*\")|(//.*)")
	content = singlelineCommentRe.ReplaceAllFunc(content, func(str []byte) []byte {
		if str[0] == '"' && str[len(str)-1] == '"' {
			return str
		}
		return []byte{}
	})
	return json.Unmarshal(content, conf)
}

type AssetServeHandler string

func (h AssetServeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := string(h)
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	info, _ := assets.AssetInfo(name)
	http.ServeContent(w, req, name, info.ModTime(), bytes.NewReader(assets.MustAsset(name)))
}

func main() {
	configFile := flag.String("conf", CONFFILE, "Path to the configuration file")
	printVersion := flag.Bool("version", false, "Print the version string")
	flag.Parse()

	if *printVersion {
		fmt.Printf("Version: %v (%v)\n", VERSION, BUILD)
		return
	}

	log.Printf("Version: %v (%v)\n", VERSION, BUILD)
	var config Config
	if err := config.Load(*configFile); err != nil {
		log.Fatal(err)
	}

	storeDir := strings.Replace(config.StorageDir, "~", os.Getenv("HOME"), 1)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		log.Fatalf("Unable to create config dir: %v", err)
	}
	log.Printf("Using \"%s\" for storage", storeDir)

	streamdb, err := stream.NewDB(path.Join(storeDir, "streams"))
	if err != nil {
		log.Fatalf("Unable to create stream database: %v", err)
	}

	filterFactories := []func() filter.Filter{
		func() filter.Filter { return &ruled.RuleFilter{} },
		func() filter.Filter { return &keyed.Query{} },
	}
	filterdb, err := filter.NewDB(path.Join(storeDir, "filters"), filterFactories...)
	if err != nil {
		log.Fatalf("Unable to create filterdb: %v", err)
	}

	players, err := connectToPlayers(&config)
	if err != nil {
		log.Fatal(err)
	}
	if len(players) == 0 {
		log.Fatal("No players configured")
	}
	defaultPlayer := config.DefaultPlayer
	if defaultPlayer == "" {
		for name := range players {
			defaultPlayer = name
			break
		}
	}
	if _, ok := players[defaultPlayer]; !ok {
		log.Fatalf("The configured default player is unknown: %q", defaultPlayer)
	}

	for name, pl := range players {
		log.Printf("Attached player %v", pl)
		cache := &player.TrackCache{Player: pl}
		players[name] = cache
		go cache.Run()
	}

	if config.AutoQueue {
		for name, pl := range players {
			go func(pl player.Player, name string) {
				ev := filterdb.Listen()
				defer filterdb.Unlisten(ev)
				for {
					ft, err := filterdb.Get("queuer")
					if err != nil {
						ft, _ = ruled.BuildFilter([]ruled.Rule{})
						if err := filterdb.Store("queuer", ft); err != nil {
							log.Printf("Error while autoqueueing for %q: %v", name, err)
						}
					}
					com := player.AutoAppend(pl, filter.RandomIterator(ft))
					select {
					case err := <-com:
						if err != nil {
							log.Printf("Error while autoqueueing for %q: %v", name, err)
						}
					case <-ev:
						com <- nil
					}
				}
			}(pl, name)
		}
	}

	fullUrlRoot, err := determineFullURLRoot(config.URLRoot, config.Address)
	if err != nil {
		log.Fatal(err)
	}
	rawServer, err := player.NewRawTrackServer(fmt.Sprintf("%sdata/raw", fullUrlRoot))
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()
	r.Handle("/", http.RedirectHandler("/player/"+defaultPlayer, http.StatusTemporaryRedirect))
	for name, pl := range players {
		func(name string, pl player.Player) {
			playerOnline := func(r *http.Request, rm *mux.RouteMatch) bool {
				return pl.Available()
			}
			r.Path(fmt.Sprintf("/player/%s", name)).MatcherFunc(playerOnline).HandlerFunc(htBrowserPage(&config, players, name))
			htPlayerDataAttach(r.PathPrefix(fmt.Sprintf("/data/player/%s/", name)).MatcherFunc(playerOnline).Subrouter(), pl, streamdb, rawServer)
		}(name, pl)
	}

	for _, file := range assets.AssetNames() {
		if !strings.HasPrefix(file, PUBLIC_DIR) {
			continue
		}
		urlPath := strings.TrimPrefix(file, PUBLIC_DIR)
		r.Path(urlPath).Handler(AssetServeHandler(file))
	}
	htDataAttach(r.PathPrefix("/data/").Subrouter(), filterdb, streamdb, rawServer)

	log.Printf("Now accepting HTTP connections on %v", config.Address)
	server := &http.Server{
		Addr:           config.Address,
		Handler:        util.Gzip(r),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatalf("Error running webserver: %v", server.ListenAndServe())
}

func connectToPlayers(config *Config) (map[string]player.Player, error) {
	players := map[string]player.Player{}
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
		mpdPlayer, err := mpd.Connect(mpdConf.Network, mpdConf.Address, mpdConf.Password)
		if err != nil {
			return nil, fmt.Errorf("Unable to connect to MPD: %v", err)
		}
		if err := addPlayer(mpdPlayer, mpdConf.Name); err != nil {
			return nil, err
		}
	}
	if config.SlimServer != nil {
		slimServ, err := slimserver.Connect(
			config.SlimServer.Network,
			config.SlimServer.Address,
			config.SlimServer.Username,
			config.SlimServer.Password,
			config.SlimServer.WebUrl,
		)
		if err != nil {
			return nil, fmt.Errorf("Unable to connect to SlimServer: %v", err)
		}
		players, err := slimServ.Players()
		if err != nil {
			return nil, err
		}
		for _, pl := range players {
			if err := addPlayer(pl, pl.Name); err != nil {
				return nil, err
			}
		}
	}
	return players, nil
}

func getStaticAssets(files []string) map[string][]string {
	static := map[string][]string{
		"js":  []string{},
		"css": []string{},
	}
	for _, file := range files {
		if !strings.HasPrefix(file, PUBLIC_DIR) {
			continue
		}
		urlPath := strings.TrimPrefix(file, PUBLIC_DIR+"/")
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

func determineFullURLRoot(root, address string) (string, error) {
	// Handle "http://host:port/"
	if regexp.MustCompile("^https?:\\/\\/").MatchString(root) {
		return root, nil
	}
	// Handle "//host:port/"
	if regexp.MustCompile("^\\/\\/.").MatchString(root) {
		// Assume plain HTTP. If you are smart enough to set up HTTPS you are
		// also smart enough to configure the URLRoot.
		return "http:" + root, nil
	}
	// Handle "/"
	if root == "/" {
		i := strings.LastIndex(address, ":")
		host, port := address[:i], address[i+1:]
		if host == "" || host == "0.0.0.0" {
			host = "127.0.0.1"
		} else if host == "[::]" {
			host = "[::1]"
		}
		return fmt.Sprintf("http://%s:%s/", host, port), nil
	}
	// Give up
	return "", fmt.Errorf("Unsupported URL Root format: %q", root)
}

func baseParamMap(config *Config, players map[string]player.Player) map[string]interface{} {
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
