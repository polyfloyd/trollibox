package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"./filter"
	"./filter/keyed"
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

func plTrackJson(tr *player.Track, meta *player.TrackMeta) interface{} {
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

		QueuedBy: meta.QueuedBy,
	}
}

func trackJsonList(inList []player.Track) (outList []interface{}) {
	outList = make([]interface{}, len(inList))
	for i, tr := range inList {
		outList[i] = trackJson(&tr)
	}
	return
}

func pltrackJsonList(inList []player.Track, meta []player.TrackMeta, libs []player.Library) ([]interface{}, error) {
	outList := make([]interface{}, len(inList))
	uris := make([]string, len(inList))
	for i, tr := range inList {
		uris[i] = tr.Uri
	}
	tracks, err := player.AllTrackInfo(libs, uris...)
	if err != nil {
		return nil, err
	}

	for i, tr := range tracks {
		outList[i] = plTrackJson(&tr, &meta[i])
	}
	return outList, nil
}

func htPlayerDataAttach(r *mux.Router, pl player.Player, streamdb *stream.DB, queuer *player.Queuer, rawServer *player.RawTrackServer) {
	libs := []player.Library{pl, streamdb, rawServer}
	r.Path("/playlist").Methods("GET").HandlerFunc(htPlayerGetPlaylist(pl, libs))
	r.Path("/playlist").Methods("PUT").HandlerFunc(htPlayerPlaylistInsert(pl))
	r.Path("/playlist").Methods("PATCH").HandlerFunc(htPlayerPlaylistMove(pl))
	r.Path("/playlist").Methods("DELETE").HandlerFunc(htPlayerPlaylistRemove(pl))
	r.Path("/playlist/appendraw").Methods("POST").HandlerFunc(htRawTrackAdd(pl, rawServer))
	r.Path("/next").Methods("POST").HandlerFunc(htPlayerNext(pl))
	r.Path("/time").Methods("GET").HandlerFunc(htPlayerGetTime(pl))
	r.Path("/time").Methods("POST").HandlerFunc(htPlayerSetTime(pl))
	r.Path("/playstate").Methods("GET").HandlerFunc(htPlayerGetPlaystate(pl))
	r.Path("/playstate").Methods("POST").HandlerFunc(htPlayerSetPlaystate(pl))
	r.Path("/volume").Methods("GET").HandlerFunc(htPlayerGetVolume(pl))
	r.Path("/volume").Methods("POST").HandlerFunc(htPlayerSetVolume(pl))
	r.Path("/tracks").Methods("GET").HandlerFunc(htPlayerTracks(pl))
	r.Path("/tracks/search").Methods("GET").HandlerFunc(htTrackSearch(pl))
	r.Path("/tracks/art").Methods("GET").HandlerFunc(htTrackArt(libs))
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
		trackIndex, err := pl.TrackIndex()
		if err != nil {
			writeError(res, err)
			return
		}
		if err := pl.SetTrackIndex(trackIndex + 1); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerSetTime(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Time int `json:"time"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := pl.SetTime(time.Duration(data.Time) * time.Second); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerGetTime(pl player.Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		tim, err := pl.Time()
		if err != nil {
			writeError(res, err)
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"time": int(tim / time.Second),
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
			"playstate": playstate,
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

		if err := pl.SetState(player.PlayState(data.State)); err != nil {
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
		tracks, err := pl.Playlist().Tracks()
		if err != nil {
			writeError(res, err)
			return
		}
		meta, err := pl.Playlist().Meta()
		if err != nil {
			writeError(res, err)
			return
		}
		trackIndex, err := pl.TrackIndex()
		if err != nil {
			writeError(res, err)
			return
		}
		tim, err := pl.Time()
		if err != nil {
			writeError(res, err)
			return
		}
		trJson, err := pltrackJsonList(tracks, meta, libs)
		if err != nil {
			writeError(res, err)
			return
		}

		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"time":    int(tim / time.Second),
			"current": trackIndex,
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

		tracks := make([]player.Track, len(data.Tracks))
		for i, uri := range data.Tracks {
			tracks[i].Uri = uri
		}
		meta := make([]player.TrackMeta, len(data.Tracks))
		for i := range data.Tracks {
			meta[i].QueuedBy = "user"
		}
		plist := pl.Playlist()
		if err := plist.InsertWithMeta(data.Pos, tracks, meta); err != nil {
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

		if err := pl.Playlist().Move(data.From, data.To); err != nil {
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

		if err := pl.Playlist().Remove(data.Positions...); err != nil {
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

		untaggedFields := strings.Split(req.FormValue("untagged"), ",")
		compiledQuery, err := keyed.CompileQuery(req.FormValue("query"), untaggedFields)
		if err != nil {
			writeError(res, err)
			return
		}
		results := filter.FilterTracks(compiledQuery, tracks)
		sort.Sort(filter.ByNumMatches(results))

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
					tracks, err := pl.Playlist().Tracks()
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

			err = pl.Playlist().InsertWithMeta(-1, []player.Track{track}, []player.TrackMeta{
				{QueuedBy: "user"},
			})
			if err != nil {
				writeError(res, err)
				return
			}
		}

		res.Write([]byte("{}"))
	}
}
