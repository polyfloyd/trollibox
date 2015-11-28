package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"./player"
	"./stream"
	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)

var httpCacheSince = time.Now()

func trackJson(tr *player.Track) interface{} {
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
		HasArt      bool   `json:"hasart"`
	}{
		Uri:         tr.Uri,
		Artist:      tr.Artist,
		Title:       tr.Title,
		Genre:       tr.Genre,
		Album:       tr.Album,
		AlbumArtist: tr.AlbumArtist,
		AlbumTrack:  tr.AlbumTrack,
		AlbumDisc:   tr.AlbumDisc,
		Duration:    int(tr.Duration / time.Second),
		HasArt:      tr.HasArt,
	}
}

func plTrackJson(plTr player.PlaylistTrack, tr *player.Track) interface{} {
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
		HasArt      bool   `json:"hasart"`

		QueuedBy string `json:"queuedby"`
		Progress int    `json:"progress"`
	}{
		Uri:         plTr.TrackUri(),
		Artist:      tr.Artist,
		Title:       tr.Title,
		Genre:       tr.Genre,
		Album:       tr.Album,
		AlbumArtist: tr.AlbumArtist,
		AlbumTrack:  tr.AlbumTrack,
		AlbumDisc:   tr.AlbumDisc,
		Duration:    int(tr.Duration / time.Second),
		HasArt:      tr.HasArt,

		QueuedBy: plTr.QueuedBy,
		Progress: int(plTr.Progress / time.Second),
	}
}

func trackJsonList(inList []player.Track) (outList []interface{}) {
	outList = make([]interface{}, len(inList))
	for i, tr := range inList {
		outList[i] = trackJson(&tr)
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
		outList[i] = plTrackJson(tr, &tracks[i])
	}
	return outList, nil
}

func htPlayerDataAttach(r *mux.Router, pl player.Player, streamdb *stream.DB) {
	r.Path("/playstate").Methods("GET").HandlerFunc(htPlayerGetPlaystate(pl))
	r.Path("/playstate").Methods("POST").HandlerFunc(htPlayerSetPlaystate(pl))
	r.Path("/volume").Methods("GET").HandlerFunc(htPlayerGetVolume(pl))
	r.Path("/volume").Methods("POST").HandlerFunc(htPlayerSetVolume(pl))
	r.Path("/playlist").Methods("GET").HandlerFunc(htPlayerGetPlaylist(pl))
	r.Path("/playlist").Methods("POST").HandlerFunc(htPlayerSetPlaylist(pl))
	r.Path("/progress").Methods("GET").HandlerFunc(htPlayerGetProgress(pl))
	r.Path("/progress").Methods("POST").HandlerFunc(htPlayerSetProgress(pl))
	r.Path("/tracks").Methods("GET").HandlerFunc(htPlayerTracks(pl))
	r.Path("/tracks/search").Methods("GET").HandlerFunc(htTrackSearch(pl))
	r.Path("/art").Methods("GET").HandlerFunc(htTrackArt(pl, streamdb))
	r.Path("/next").Methods("POST").HandlerFunc(htPlayerNext(pl))
	r.Path("/listen").Handler(websocket.Handler(htPlayerListen(pl)))
}

func htPlayerListen(pl player.Player) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		ch := pl.Events().Listen()
		defer pl.Events().Unlisten(ch)

		conn.SetDeadline(time.Time{})
		for {
			_, err := conn.Write([]uint8(<-ch))
			if err != nil {
				break
			}
		}
	}
}

func htPlayerNext(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		if err := player.PlaylistNext(pl); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerSetProgress(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Progress int `json:"progress"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := pl.Seek(time.Duration(data.Progress) * time.Second); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerGetProgress(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		plist, err := pl.Playlist()
		if err != nil {
			writeError(res, err)
			return
		}

		var progress time.Duration
		if len(plist) > 0 {
			progress = plist[0].Progress
		} else {
			progress = 0
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"progress": int(progress / time.Second),
		})
	}
}

func htPlayerGetPlaystate(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		playstate, err := pl.State()
		if err != nil {
			writeError(res, err)
			return
		}

		json.NewEncoder(res).Encode(map[string]interface{}{
			"playstate": playstate.Name(),
		})
	}
}

func htPlayerSetPlaystate(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			State string `json:"playstate"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := pl.SetState(player.NamedPlaystate(data.State)); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerGetVolume(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		volume, err := pl.Volume()
		if err != nil {
			writeError(res, err)
			return
		}

		json.NewEncoder(res).Encode(map[string]interface{}{
			"volume": volume,
		})
	}
}

func htPlayerSetVolume(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Volume float32 `json:"volume"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := pl.SetVolume(data.Volume); err != nil {
			writeError(res, err)
			return
		}

		res.Write([]byte("{}"))
	}
}

func htPlayerGetPlaylist(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		tracks, err := pl.Playlist()
		if err != nil {
			writeError(res, err)
			return
		}
		trJson, err := pltrackJsonList(tracks, pl)
		if err != nil {
			writeError(res, err)
			return
		}

		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": trJson,
		})
		if err != nil {
			writeError(res, err)
			return
		}
	}
}

func htPlayerSetPlaylist(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			TrackIds []string `json:"track-ids"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := player.SetPlaylistIds(pl, player.TrackIdentities(data.TrackIds...)); err != nil {
			writeError(res, err)
			return
		}

		res.Write([]byte("{}"))
	}
}

func htPlayerTracks(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	var cache bytes.Buffer
	var cacheMutex sync.RWMutex
	var err error

	go func() {
		listener := pl.Events().Listen()
		defer pl.Events().Unlisten(listener)
		listener <- "tracks" // Bootstrap the cycle.

		for {
			if event := <-listener; event != "tracks" {
				continue
			}
			cacheMutex.Lock()
			var tracks []player.Track
			tracks, err = pl.TrackInfo()
			cache.Reset()
			json.NewEncoder(&cache).Encode(map[string]interface{}{
				"tracks": trackJsonList(tracks),
			})
			cacheMutex.Unlock()
		}
	}()

	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		if err != nil {
			writeError(res, err)
			return
		}
		cacheMutex.RLock()
		res.Write(cache.Bytes())
		cacheMutex.RUnlock()
	}
}

func htTrackArt(pl player.Player, streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		uri := req.FormValue("track")

		var image io.ReadCloser
		var mime string
		if stream := streamdb.StreamByURL(uri); stream != nil && stream.ArtUrl != "" {
			image, mime = stream.Art()
		} else {
			tracks, err := pl.TrackInfo(player.TrackIdentities(uri)...)
			if err != nil {
				writeError(res, err)
				return
			}
			if len(tracks) > 0 && tracks[0].HasArt {
				image, mime = pl.TrackArt(tracks[0])
			}
		}
		if image == nil {
			http.NotFound(res, req)
			return
		}

		defer image.Close()

		res.Header().Set("Content-Type", mime)
		if req.Method == "HEAD" {
			return
		}
		var buf bytes.Buffer
		io.Copy(&buf, image)
		http.ServeContent(res, req, path.Base(uri), httpCacheSince, bytes.NewReader(buf.Bytes()))
	}
}

func htTrackSearch(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	var cachedTracks []player.Track
	var cacheMutex sync.RWMutex
	var err error

	go func() {
		listener := pl.Events().Listen()
		defer pl.Events().Unlisten(listener)
		listener <- "tracks" // Bootstrap the cycle.

		for {
			if event := <-listener; event != "tracks" {
				continue
			}
			cacheMutex.Lock()
			cachedTracks, err = pl.TrackInfo()
			cacheMutex.Unlock()
		}
	}()

	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		if err != nil {
			writeError(res, err)
			return
		}

		cacheMutex.RLock()
		untagged := strings.Split(req.FormValue("untagged"), ",")
		results, err := player.Search(cachedTracks, req.FormValue("query"), untagged)
		cacheMutex.RUnlock()
		if err != nil {
			writeError(res, err)
			return
		}

		mappedResults := make([]interface{}, len(results))
		for i, res := range results {
			mappedResults[i] = map[string]interface{}{
				"matches": res.Matches,
				"track":   trackJson(&res.Track),
			}
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": mappedResults,
		})
	}
}
