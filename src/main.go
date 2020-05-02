package main

import (
	"bytes"
	"flag"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/polyfloyd/trollibox/src/api"
	"github.com/polyfloyd/trollibox/src/assets"
	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/ruled"
	"github.com/polyfloyd/trollibox/src/jukebox"
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
	confFile  = "config.yaml"
)

var (
	build       = "%BUILD%"
	version     = "%VERSION%"
	versionDate = "%VERSION_DATE%"
	buildDate   = "%BUILD_DATE%"
)

var static = getStaticAssets(assets.AssetNames())

type config struct {
	Address string `yaml:"bind"`
	URLRoot string `yaml:"url_root"`

	StorageDir string `yaml:"storage_dir"`

	AutoQueue     bool   `yaml:"autoqueue"`
	DefaultPlayer string `yaml:"default_player"`

	Colors struct {
		Background     string `yaml:"background"`
		BackgroundElem string `yaml:"background_elem"`
		Text           string `yaml:"text"`
		TextInactive   string `yaml:"text_inactive"`
		Accent         string `yaml:"accent"`
	} `yaml:"colors"`

	MPD []struct {
		Name     string  `yaml:"name"`
		Network  string  `yaml:"network"`
		Address  string  `yaml:"address"`
		Password *string `yaml:"password"`
	} `yaml:"mpd"`

	SlimServer *struct {
		Network  string  `yaml:"network"`
		Address  string  `yaml:"address"`
		Username *string `yaml:"username"`
		Password *string `yaml:"password"`
		WebURL   string  `yaml:"weburl"`
	} `yaml:"slimserver"`
}

func (conf *config) Validate() (errs []error) {
	if conf.Address == "" {
		errs = append(errs, fmt.Errorf("config: `bind` is required"))
	}
	if len(conf.MPD) == 0 && conf.SlimServer == nil {
		errs = append(errs, fmt.Errorf("config: no media servers configured"))
	}
	return
}

func LoadConfig(filename string) (*config, error) {
	fd, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	d := yaml.NewDecoder(fd)
	d.KnownFields(true)
	var conf config
	if err := d.Decode(&conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

type assetServeHandler string

func (h assetServeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := string(h)
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	info, _ := assets.AssetInfo(name)
	http.ServeContent(w, req, name, info.ModTime(), bytes.NewReader(assets.MustAsset(name)))
}

func htDefaultAlbumArt(config *config) http.HandlerFunc {
	filename := "default-album-art.svg"
	recolored := bytes.Replace(assets.MustAsset(filename), []byte("#ffffff"), []byte(config.Colors.Accent), -1)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		info, _ := assets.AssetInfo(filename)
		http.ServeContent(w, req, filename, info.ModTime(), bytes.NewReader(recolored))
	})
}

func main() {
	defaultLogLevel := "warn"
	if build == "debug" {
		defaultLogLevel = "debug"
	}

	configFile := flag.String("conf", confFile, "Path to the configuration file")
	printVersion := flag.Bool("version", false, "Print version information and exit")
	logLevel := flag.String("log", defaultLogLevel, "Sets the log level. [debug, info, warn, error]")
	flag.Parse()

	if ll, err := log.ParseLevel(*logLevel); err != nil {
		log.Fatalf("Could not parse log level: %v", err)
	} else {
		log.SetLevel(ll)
	}
	log.SetReportCaller(true)

	if *printVersion {
		fmt.Printf("Version: %v (%v)\n", version, versionDate)
		fmt.Printf("Build: %v\n", build)
		fmt.Printf("Build TIme: %v\n", buildDate)
		return
	}

	log.Infof("Version: %v (%v)\n", version, build)
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}
	if errs := config.Validate(); len(errs) > 0 {
		log.Fatalf("Could not load config: %v", errs)
	}

	storeDir := strings.Replace(config.StorageDir, "~", os.Getenv("HOME"), 1)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		log.Fatalf("Unable to create config dir: %v", err)
	}
	log.Infof("Using %q for storage", storeDir)

	streamdb, err := stream.NewDB(path.Join(storeDir, "streams"))
	if err != nil {
		log.Fatalf("Unable to create stream database: %v", err)
	}

	filterdb, err := filter.NewDB(path.Join(storeDir, "filters"))
	if err != nil {
		log.Fatalf("Unable to create filterdb: %v", err)
	}

	players, err := connectToPlayers(config)
	if err != nil {
		log.Fatal(err)
	}
	if names, err := players.PlayerNames(); err != nil {
		log.Fatal(err)
	} else if len(names) == 0 {
		log.Fatal("No players configured or available")
	} else {
		for _, name := range names {
			log.Infof("Found player %q", name)
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

	jukebox := jukebox.NewJukebox(players, netServer, filterdb, streamdb, rawServer)

	service := chi.NewRouter()
	service.Use(util.LogHandler)
	service.Use(middleware.DefaultCompress)
	for _, file := range assets.AssetNames() {
		if !strings.HasPrefix(file, publicDir) {
			continue
		}
		urlPath := strings.TrimPrefix(file, publicDir)
		service.Get(urlPath, assetServeHandler(file).ServeHTTP)
	}
	service.Get("/img/default-album-art.svg", htDefaultAlbumArt(config))

	service.Get("/", htRedirectToDefaultPlayer(config, players))
	service.Get("/player/{player}", htBrowserPage(config, players))
	service.Route("/data", func(r chi.Router) {
		api.InitRouter(r, jukebox)
	})

	log.Infof("Now accepting HTTP connections on %v", config.Address)
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
		log.Errorf("error attaching autoqueuer: %v", err)
		return
	}
	for _, name := range names {
		pl, err := players.PlayerByName(name)
		if err != nil {
			log.WithField("player", name).Errorf("Error attaching autoqueuer: %v", err)
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
						log.WithField("player", name).Errorf("Error while autoqueueing: %v", err)
					}
				}
				cancel := make(chan struct{})
				com := player.AutoAppend(pl, filter.RandomIterator(ft), cancel)
				select {
				case err := <-com:
					if err != nil {
						log.WithField("player", name).Errorf("Error while autoqueueing: %v", err)
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
	for _, mpdConf := range config.MPD {
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
		"colors": map[string]string{
			"bg":           config.Colors.Background,
			"bgElem":       config.Colors.BackgroundElem,
			"text":         config.Colors.Text,
			"textInactive": config.Colors.TextInactive,
			"accent":       config.Colors.Accent,
		},
	}
}
