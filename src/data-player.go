package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"
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
		Uri         string `json:"uri"`
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

func plTrackJson(tr *player.PlaylistTrack) interface{} {
	return &struct {
		Uri         string `json:"uri"`
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

		QueuedBy: tr.QueuedBy,
		Progress: int(tr.Progress / time.Second),
	}
}

func trackJsonList(inList []player.Track) (outList []interface{}) {
	outList = make([]interface{}, len(inList))
	for i, tr := range inList {
		outList[i] = trackJson(&tr)
	}
	return
}

func pltrackJsonList(inList []player.PlaylistTrack, libs []player.Library) ([]interface{}, error) {
	outList := make([]interface{}, len(inList))
	uris := make([]string, len(inList))
	for i, tr := range inList {
		uris[i] = tr.Uri
	}
	tracks, err := player.AllTrackInfo(libs, uris...)
	if err != nil {
		return nil, err
	}

	for i, tr := range inList {
		tr.Track = tracks[i]
		outList[i] = plTrackJson(&tr)
	}
	return outList, nil
}

func htPlayerDataAttach(r *mux.Router, pl player.Player, streamdb *stream.DB, queuer *player.Queuer, rawServer *player.RawTrackServer) {
	libs := []player.Library{pl, streamdb, rawServer}
	r.Path("/playstate").Methods("GET").HandlerFunc(htPlayerGetPlaystate(pl))
	r.Path("/playstate").Methods("POST").HandlerFunc(htPlayerSetPlaystate(pl))
	r.Path("/volume").Methods("GET").HandlerFunc(htPlayerGetVolume(pl))
	r.Path("/volume").Methods("POST").HandlerFunc(htPlayerSetVolume(pl))
	r.Path("/playlist").Methods("GET").HandlerFunc(htPlayerGetPlaylist(pl, libs))
	r.Path("/playlist").Methods("PUT").HandlerFunc(htPlayerPlaylistInsert(pl))
	r.Path("/playlist").Methods("PATCH").HandlerFunc(htPlayerPlaylistMove(pl))
	r.Path("/playlist").Methods("DELETE").HandlerFunc(htPlayerPlaylistRemove(pl))
	r.Path("/progress").Methods("GET").HandlerFunc(htPlayerGetProgress(pl))
	r.Path("/progress").Methods("POST").HandlerFunc(htPlayerSetProgress(pl))
	r.Path("/tracks").Methods("GET").HandlerFunc(htPlayerTracks(pl))
	r.Path("/tracks/search").Methods("GET").HandlerFunc(htTrackSearch(pl))
	r.Path("/art").Methods("GET").HandlerFunc(htTrackArt(libs))
	r.Path("/next").Methods("POST").HandlerFunc(htPlayerNext(pl))
	r.Path("/appendraw").Methods("POST").HandlerFunc(htRawTrackAdd(pl, rawServer))
	r.Path("/listen").Handler(websocket.Handler(htPlayerListen(pl, streamdb, queuer)))
}

func htPlayerListen(pl player.Player, streamdb *stream.DB, queuer *player.Queuer) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		plCh := pl.Events().Listen()
		defer pl.Events().Unlisten(plCh)
		strCh := streamdb.Listen()
		defer streamdb.Unlisten(strCh)
		quCh := queuer.Listen()
		defer queuer.Unlisten(quCh)

		conn.SetDeadline(time.Time{})
		for {
			var event string
			select {
			case event = <-plCh:
			case ev := <-strCh:
				event = "streams-" + ev
			case ev := <-quCh:
				event = "queuer-" + ev
			}
			if _, err := conn.Write([]uint8(event)); err != nil {
				break
			}
		}
	}
}

func htPlayerNext(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		_, currentTrackIndex, err := pl.Playlist()
		if err != nil {
			writeError(res, err)
			return
		}
		if err := pl.Seek(currentTrackIndex+1, -1); err != nil {
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

		if err := pl.Seek(-1, time.Duration(data.Progress)*time.Second); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerGetProgress(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		plist, currentTrackIndex, err := pl.Playlist()
		if err != nil {
			writeError(res, err)
			return
		}
		tracks, err := plist.Tracks()
		if err != nil {
			writeError(res, err)
			return
		}

		var progress time.Duration
		if len(tracks) > 0 && currentTrackIndex >= 0 {
			progress = tracks[currentTrackIndex].Progress
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

func htPlayerGetPlaylist(pl player.Player, libs []player.Library) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		plist, currentTrackIndex, err := pl.Playlist()
		if err != nil {
			writeError(res, err)
			return
		}
		tracks, err := plist.Tracks()
		if err != nil {
			writeError(res, err)
			return
		}
		trJson, err := pltrackJsonList(tracks, libs)
		if err != nil {
			writeError(res, err)
			return
		}

		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"current": currentTrackIndex,
			"tracks":  trJson,
		})
		if err != nil {
			writeError(res, err)
			return
		}
	}
}

func htPlayerPlaylistInsert(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Pos    int      `json:"position"`
			Tracks []string `json:"tracks"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		tracks := make([]player.PlaylistTrack, len(data.Tracks))
		for i, uri := range data.Tracks {
			tracks[i].Uri = uri
			tracks[i].QueuedBy = "user"
		}
		plist, _, _ := pl.Playlist()
		if err := plist.Insert(data.Pos, tracks...); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerPlaylistMove(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			From int `json:"from"`
			To   int `json:"to"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		plist, _, _ := pl.Playlist()
		if err := plist.Move(data.From, data.To); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerPlaylistRemove(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Positions []int `json:"positions"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		plist, _, _ := pl.Playlist()
		if err := plist.Remove(data.Positions...); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerTracks(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		tracks, err := pl.Tracks()
		if err != nil {
			writeError(res, err)
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": trackJsonList(tracks),
		})
	}
}

func htTrackArt(libs []player.Library) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		uri := req.FormValue("track")
		var image io.ReadCloser
		var mime string
		for _, lib := range libs {
			if image, mime = lib.TrackArt(uri); image != nil {
				break
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
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		tracks, err := pl.Tracks()
		if err != nil {
			writeError(res, err)
			return
		}

		untagged := strings.Split(req.FormValue("untagged"), ",")
		results, err := player.Search(tracks, req.FormValue("query"), untagged)
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

func htRawTrackAdd(pl player.Player, rawServer *player.RawTrackServer) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		mpReader, err := req.MultipartReader()
		if err != nil {
			writeError(res, err)
			return
		}
		plist, _, err := pl.Playlist()
		if err != nil {
			writeError(res, err)
			return
		}

		for {
			part, err := mpReader.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				writeError(res, err)
				return
			}
			// Make the file available through the server.
			track, err := rawServer.Add(part, part.FileName())
			if err != nil {
				writeError(res, err)
				return
			}

			// Launch a goroutine that will check whether the track is still in
			// the player's playlist. If it is not, the track is removed from
			// the server.
			go func(track player.Track) {
				events := pl.Events().Listen()
				defer pl.Events().Unlisten(events)
				for event := range events {
					if event != "playlist" {
						continue
					}
					plist, _, err := pl.Playlist()
					if err != nil {
						break
					}
					tracks, err := plist.Tracks()
					if err != nil {
						break
					}
					found := false
					for _, plTrack := range tracks {
						if track.Uri == plTrack.Uri {
							found = true
						}
					}
					if !found {
						break
					}
				}
				rawServer.Remove(track)
			}(track)

			err = plist.Insert(-1, player.PlaylistTrack{
				Track:    track,
				QueuedBy: "user",
			})
			if err != nil {
				writeError(res, err)
				return
			}
		}

		res.Write([]byte("{}"))
	}
}
