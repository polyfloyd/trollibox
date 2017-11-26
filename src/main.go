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

	"github.com/gorilla/mux"

	assets "github.com/polyfloyd/trollibox/src/assets-go"
	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/keyed"
	"github.com/polyfloyd/trollibox/src/filter/ruled"
	"github.com/polyfloyd/trollibox/src/library/cache"
	"github.com/polyfloyd/trollibox/src/library/netmedia"
	"github.com/polyfloyd/trollibox/src/library/raw"
	"github.com/polyfloyd/trollibox/src/library/stream"
	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/player/mpd"
	"github.com/polyfloyd/trollibox/src/player/slimserver"
	"github.com/polyfloyd/trollibox/src/util"
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

type PlayerList interface {
	ActivePlayers() []string

	ActivePlayerByName(name string) player.Player
}

type TODOPlayerList map[string]player.Player

func (list TODOPlayerList) ActivePlayers() []string {
	names := make([]string, 0, len(list))
	for name, pl := range list {
		if pl.Available() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (list TODOPlayerList) ActivePlayerByName(name string) player.Player {
	if pl, ok := list[name]; ok && pl.Available() {
		return pl
	}
	return nil
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
	if len(players.ActivePlayers()) == 0 {
		log.Fatal("No players configured or available")
	}

	if config.AutoQueue {
		// TODO: Currently, only players which are active at startup attached
		// to a queuer.
		for _, name := range players.ActivePlayers() {
			pl := players.ActivePlayerByName(name)
			go func(pl player.Player, name string) {
				filterEvents := filterdb.Listen()
				defer filterdb.Unlisten(filterEvents)
				for {
					ft, _ := filterdb.Get("queuer")
					if ft == nil {
						// Load the default filter.
						ft, _ = ruled.BuildFilter([]ruled.Rule{})
						if err := filterdb.Store("queuer", ft); err != nil {
							log.Printf("Error while autoqueueing for %q: %v", name, err)
						}
					}
					cancel := make(chan struct{})
					com := player.AutoAppend(pl, filter.RandomIterator(ft), cancel)
					select {
					case err := <-com:
						if err != nil {
							log.Printf("Error while autoqueueing for %q: %v", name, err)
						}
					case <-filterEvents:
					}
					close(cancel)
				}
			}(pl, name)
		}
	}

	fullUrlRoot, err := util.DetermineFullURLRoot(config.URLRoot, config.Address)
	if err != nil {
		log.Fatal(err)
	}
	rawServer := raw.NewServer(fmt.Sprintf("%sdata/raw", fullUrlRoot))
	netServer, err := netmedia.NewServer(rawServer)
	if err != nil {
		log.Fatal(err)
	}

	service := mux.NewRouter()
	for _, file := range assets.AssetNames() {
		if !strings.HasPrefix(file, PUBLIC_DIR) {
			continue
		}
		urlPath := strings.TrimPrefix(file, PUBLIC_DIR)
		service.Path(urlPath).Handler(AssetServeHandler(file))
	}

	service.HandleFunc("/", htRedirectToDefaultPlayer(&config, players))
	service.Path("/player/{player}").HandlerFunc(htBrowserPage(&config, players))
	dataService := service.PathPrefix("/data/").Subrouter()
	htDataAttach(dataService, filterdb, streamdb, rawServer)
	htPlayerDataAttach(dataService.PathPrefix("/player/{player}/").Subrouter(), players, streamdb, rawServer, netServer)

	log.Printf("Now accepting HTTP connections on %v", config.Address)
	server := &http.Server{
		Addr:           config.Address,
		Handler:        util.Gzip(service),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatalf("Error running webserver: %v", server.ListenAndServe())
}

func connectToPlayers(config *Config) (PlayerList, error) {
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

	for name, pl := range players {
		log.Printf("Attached player %v", pl)
		cache := &cache.Cache{Player: pl}
		players[name] = cache
		go cache.Run()
	}
	return TODOPlayerList(players), nil
}

func getStaticAssets(files []string) map[string][]string {
	static := map[string][]string{
		"js":  {},
		"css": {},
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

func baseParamMap(config *Config, players PlayerList) map[string]interface{} {
	playerNames := players.ActivePlayers()
	return map[string]interface{}{
		"urlroot": config.URLRoot,
		"version": VERSION,
		"assets":  static,
		"time":    time.Now(),
		"players": playerNames,
	}
}
