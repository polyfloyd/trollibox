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

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	"github.com/polyfloyd/trollibox/src/api"
	"github.com/polyfloyd/trollibox/src/assets"
	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/ruled"
	"github.com/polyfloyd/trollibox/src/library/netmedia"
	"github.com/polyfloyd/trollibox/src/library/raw"
	"github.com/polyfloyd/trollibox/src/library/stream"
	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/player/mpd"
	"github.com/polyfloyd/trollibox/src/player/slimserver"
	"github.com/polyfloyd/trollibox/src/util"
)

const (
	publicDir = "public"
	confFile  = "config.json"
)

var (
	build       = "%BUILD%"
	version     = "%VERSION%"
	versionDate = "%VERSION_DATE%"
	buildDate   = "%BUILD_DATE%"
)

var static = getStaticAssets(assets.AssetNames())

type config struct {
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
		WebURL   string  `json:"weburl"`
	} `json:"slimserver"`
}

func (conf *config) Load(filename string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("unable to decode config: %v", err)
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

type assetServeHandler string

func (h assetServeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := string(h)
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	info, _ := assets.AssetInfo(name)
	http.ServeContent(w, req, name, info.ModTime(), bytes.NewReader(assets.MustAsset(name)))
}

func main() {
	configFile := flag.String("conf", confFile, "Path to the configuration file")
	printVersion := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *printVersion {
		fmt.Printf("Version: %v (%v)\n", version, versionDate)
		fmt.Printf("Build: %v\n", build)
		fmt.Printf("Build TIme: %v\n", buildDate)
		return
	}

	log.Printf("Version: %v (%v)\n", version, build)
	var config config
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

	filterdb, err := filter.NewDB(path.Join(storeDir, "filters"))
	if err != nil {
		log.Fatalf("Unable to create filterdb: %v", err)
	}

	players, err := connectToPlayers(&config)
	if err != nil {
		log.Fatal(err)
	}
	if names, err := players.PlayerNames(); err != nil {
		log.Fatal(err)
	} else if len(names) == 0 {
		log.Fatal("No players configured or available")
	} else {
		for _, name := range names {
			log.Printf("Found player %q", name)
		}
	}

	if config.AutoQueue {
		// TODO: Currently, only players which are active at startup attached
		// to a queuer.
		attachAutoQueuer(players, filterdb)
	}

	fullURLRoot, err := util.DetermineFullURLRoot(config.URLRoot, config.Address)
	if err != nil {
		log.Fatal(err)
	}
	rawServer := raw.NewServer(fmt.Sprintf("%sdata/raw", fullURLRoot))
	netServer, err := netmedia.NewServer(rawServer)
	if err != nil {
		log.Fatal(err)
	}

	service := chi.NewRouter()
	service.Use(middleware.DefaultCompress)
	for _, file := range assets.AssetNames() {
		if !strings.HasPrefix(file, publicDir) {
			continue
		}
		urlPath := strings.TrimPrefix(file, publicDir)
		service.Get(urlPath, assetServeHandler(file).ServeHTTP)
	}

	service.Get("/", htRedirectToDefaultPlayer(&config, players))
	service.Get("/player/{player}", htBrowserPage(&config, players))
	service.Route("/data", func(r chi.Router) {
		api.InitRouter(r, players, netServer, filterdb, streamdb, rawServer)
	})

	log.Printf("Now accepting HTTP connections on %v", config.Address)
	server := &http.Server{
		Addr:           config.Address,
		Handler:        service,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatalf("Error running webserver: %v", server.ListenAndServe())
}

func attachAutoQueuer(players player.List, filterdb *filter.DB) {
	names, err := players.PlayerNames()
	if err != nil {
		log.Printf("error attaching autoqueuer: %v", err)
		return
	}
	for _, name := range names {
		pl, err := players.PlayerByName(name)
		if err != nil {
			log.Printf("Error attaching autoqueuer to player %q: %v", name, err)
			continue
		}
		go func(pl player.Player, name string) {
			filterEvents := filterdb.Listen()
			defer filterdb.Unlisten(filterEvents)
			for {
				ft, _ := filterdb.Get("queuer")
				if ft == nil {
					// Load the default filter.
					ft, _ = ruled.BuildFilter([]ruled.Rule{})
					if err := filterdb.Set("queuer", ft); err != nil {
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

func connectToPlayers(config *config) (player.List, error) {
	mpdPlayers := player.SimpleList{}
	for _, mpdConf := range config.Mpd {
		mpdPlayer, err := mpd.Connect(mpdConf.Network, mpdConf.Address, mpdConf.Password)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to MPD: %v", err)
		}
		if _, ok := mpdPlayers[mpdConf.Name]; ok {
			return nil, fmt.Errorf("duplicate player name: %q", mpdConf.Name)
		}
		mpdPlayers.Set(mpdConf.Name, mpdPlayer)
	}

	if config.SlimServer != nil {
		slimServ, err := slimserver.Connect(
			config.SlimServer.Network,
			config.SlimServer.Address,
			config.SlimServer.Username,
			config.SlimServer.Password,
			config.SlimServer.WebURL,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to SlimServer: %v", err)
		}
		return player.MultiList{mpdPlayers, slimServ}, nil
	}

	return mpdPlayers, nil
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

func baseParamMap(config *config, players player.List) map[string]interface{} {
	playerNames, _ := players.PlayerNames()
	return map[string]interface{}{
		"urlroot": config.URLRoot,
		"version": version,
		"assets":  static,
		"time":    time.Now(),
		"players": playerNames,
	}
}
