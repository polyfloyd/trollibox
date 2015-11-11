package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"./player"
	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)

func trackJson(tr player.Track) interface{} {
	if tr == nil {
		return nil
	}
	return &struct {
		Uri         string `json:"id"`
		Artist      string `json:"artist,omitempty"`
		Title       string `json:"title,omitempty"`
		Genre       string `json:"genre,omitempty"`
		Album       string `json:"album,omitempty"`
		AlbumArtist string `json:"albumartist,omitempty"`
		AlbumTrack  string `json:"albumtrack,omitempty"`
		AlbumDisc   string `json:"albumdisc,omitempty"`
		Duration    int    `json:"duration"`
	}{
		Uri:         tr.Uri(),
		Artist:      tr.Artist(),
		Title:       tr.Title(),
		Genre:       tr.Genre(),
		Album:       tr.Album(),
		AlbumArtist: tr.AlbumArtist(),
		AlbumTrack:  tr.AlbumTrack(),
		AlbumDisc:   tr.AlbumDisc(),
		Duration:    int(tr.Duration() / time.Second),
	}
}

func plTrackJson(plTr player.PlaylistTrack, tr player.Track) interface{} {
	return &struct {
		Uri         string `json:"id"`
		Artist      string `json:"artist,omitempty"`
		Title       string `json:"title,omitempty"`
		Genre       string `json:"genre,omitempty"`
		Album       string `json:"album,omitempty"`
		AlbumArtist string `json:"albumartist,omitempty"`
		AlbumTrack  string `json:"albumtrack,omitempty"`
		AlbumDisc   string `json:"albumdisc,omitempty"`
		Duration    int    `json:"duration"`

		QueuedBy string `json:"queuedby,omitempty"`
		Progress int    `json:"progress,omitempty"`
	}{
		Uri:         plTr.Uri(),
		Artist:      tr.Artist(),
		Title:       tr.Title(),
		Genre:       tr.Genre(),
		Album:       tr.Album(),
		AlbumArtist: tr.AlbumArtist(),
		AlbumTrack:  tr.AlbumTrack(),
		AlbumDisc:   tr.AlbumDisc(),
		Duration:    int(tr.Duration() / time.Second),

		QueuedBy: plTr.QueuedBy,
		Progress: int(plTr.Progress / time.Second),
	}
}

func trackJsonList(inList []player.Track) (outList []interface{}) {
	outList = make([]interface{}, len(inList))
	for i, tr := range inList {
		outList[i] = trackJson(tr)
	}
	return
}

func pltrackJsonList(inList []player.PlaylistTrack, pl player.Player) ([]interface{}, error) {
	outList := make([]interface{}, len(inList))
	ids := make([]player.TrackIdentity, len(inList))
	for i, id := range inList {
		ids[i] = id
	}

	tracks, err := pl.TrackInfo(ids...)
	if err != nil {
		return nil, err
	}

	for i, tr := range inList {
		outList[i] = plTrackJson(tr, tracks[i])
	}
	return outList, nil
}

func socketHandler(pl player.Player) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		ch := pl.Events().Listen()
		defer pl.Events().Unlisten(ch)

		conn.SetDeadline(time.Now().Add(time.Hour * 42 * 42 * 42))

		for {
			_, err := conn.Write([]uint8(<-ch))
			if err != nil {
				break
			}
		}
	}
}

func htPlayerDataAttach(r *mux.Router, pl player.Player, streamdb *player.StreamDB) {
	r.Path("/state").Methods("POST").HandlerFunc(htPlayerSetState(pl))
	r.Path("/next").Methods("POST").HandlerFunc(htPlayerNext(pl))
	r.Path("/progress").Methods("POST").HandlerFunc(htPlayerProgress(pl))
	r.Path("/volume").Methods("GET").HandlerFunc(htPlayerGetVolume(pl))
	r.Path("/volume").Methods("POST").HandlerFunc(htPlayerSetVolume(pl))
	r.Path("/playlist").Methods("GET").HandlerFunc(htPlayerGetPlaylist(pl))
	r.Path("/playlist").Methods("POST").HandlerFunc(htPlayerSetPlaylist(pl))
	r.Path("/current").Methods("GET").HandlerFunc(htPlayerCurrentTrack(pl))
	r.Path("/browse{path:.*}").Methods("GET").HandlerFunc(htPlayerTracks(pl))
	r.Path("/art/{path:.*}").Methods("GET").HandlerFunc(htTrackArt(pl, streamdb))
	r.Path("/art/{path:.*}").Methods("HEAD").HandlerFunc(htTrackArtProbe(pl, streamdb))
	r.Path("/listen").Handler(websocket.Handler(socketHandler(pl)))
}

func htPlayerNext(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		if err := pl.Next(); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		res.Write([]byte("{}"))
	}
}

func htPlayerProgress(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Progress int `json:"progress"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := pl.Seek(time.Duration(data.Progress) * time.Second); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		res.Write([]byte("{}"))
	}
}

func htPlayerSetState(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			State string `json:"state"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := pl.SetState(player.NamedPlaystate(data.State)); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		res.Write([]byte("{}"))
	}
}

func htPlayerGetVolume(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		volume, err := pl.Volume()
		if err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		json.NewEncoder(res).Encode(map[string]interface{}{
			"volume": volume,
		})
	}
}

func htPlayerSetVolume(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Volume float32 `json:"volume"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := pl.SetVolume(data.Volume); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		res.Write([]byte("{}"))
	}
}

// DEPRECATED: Just query the playlist instead.
func htPlayerCurrentTrack(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		tracks, err := pl.Playlist()
		if err != nil {
			panic(err)
		}
		if len(tracks) > 0 {
			tracks = tracks[0:1]
		}
		trJson, err := pltrackJsonList(tracks, pl)
		if err != nil {
			panic(err)
		}
		state, err := pl.State()
		if err != nil {
			panic(err)
		}

		if len(tracks) == 0 {
			json.NewEncoder(res).Encode(map[string]interface{}{
				"progress": 0,
				"track":    nil,
				"state":    state.Name(),
			})
		} else {
			json.NewEncoder(res).Encode(map[string]interface{}{
				"progress": int(tracks[0].Progress / time.Second),
				"track":    trJson[0],
				"state":    state.Name(),
			})
		}
	}
}

func htPlayerGetPlaylist(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		tracks, err := pl.Playlist()
		if err != nil {
			panic(err)
		}
		trJson, err := pltrackJsonList(tracks, pl)
		if err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": trJson,
		})
		if err != nil {
			panic(err)
		}
	}
}

func htPlayerSetPlaylist(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			TrackIds []string `json:"track-ids"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := player.SetPlaylistIds(pl, player.TrackIdentities(data.TrackIds...)); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		res.Write([]byte("{}"))
	}
}

func htPlayerTracks(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	getResponse := func(path string, wr io.Writer) error {
		tracks, err := pl.TrackInfo()
		if err != nil {
			return err
		}

		return json.NewEncoder(wr).Encode(map[string]interface{}{
			"tracks": trackJsonList(tracks),
		})
	}

	// Only cache the root since it is the most commonly requested path.
	var cachedRoot *bytes.Buffer
	var cacheMutex sync.Mutex
	go func() {
		listener := pl.Events().Listen()
		defer pl.Events().Unlisten(listener)
		listener <- "update" // Bootstrap the cycle

		for {
			if event := <-listener; event != "update" {
				continue
			}

			cacheMutex.Lock()
			var newCachedRoot bytes.Buffer
			getResponse("/", &newCachedRoot)
			cachedRoot = &newCachedRoot
			cacheMutex.Unlock()
		}
	}()

	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		if path := mux.Vars(req)["path"]; path == "" || path == "/" {
			cacheMutex.Lock()
			buf := cachedRoot
			cacheMutex.Unlock()
			res.Write(buf.Bytes())
		} else {
			if err := getResponse(path, res); err != nil {
				panic(err)
			}
		}
	}
}

func htTrackArt(pl player.Player, streamdb *player.StreamDB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		uri := mux.Vars(req)["path"]

		var track player.Track
		if stream := streamdb.StreamByURL(fixStreamUri(uri)); stream != nil {
			track = stream
		} else {
			tracks, err := pl.TrackInfo(player.TrackIdentities(uri)[0])
			if err != nil {
				panic(err)
			}
			if len(tracks) > 0 {
				track = tracks[0]
			}
		}

		if artStream, mime := track.Art(); artStream != nil {
			res.Header().Set("Content-Type", mime)
			io.Copy(res, artStream)
			return
		}

		http.NotFound(res, req)
	}
}

func htTrackArtProbe(pl player.Player, streamdb *player.StreamDB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		uri := mux.Vars(req)["path"]

		var track player.Track
		if stream := streamdb.StreamByURL(fixStreamUri(uri)); stream != nil {
			track = stream
		} else {
			tracks, err := pl.TrackInfo(player.TrackIdentities(uri)[0])
			if err == nil {
				if len(tracks) > 0 {
					track = tracks[0]
				}
			}
		}

		trackHasArt := func(track player.Track) bool {
			image, _ := track.Art()
			if image != nil {
				image.Close()
				return true
			}
			return false
		}
		if track == nil || !trackHasArt(track) {
			http.NotFound(res, req)
		}
	}
}
