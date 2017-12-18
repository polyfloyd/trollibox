package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"

	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/keyed"
	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/library/netmedia"
	"github.com/polyfloyd/trollibox/src/library/raw"
	"github.com/polyfloyd/trollibox/src/library/stream"
	"github.com/polyfloyd/trollibox/src/player"
)

var httpCacheSince = time.Now()
var playerContextKey = playerContextType{}

type playerContextType struct{}

func trackJSON(tr *library.Track, meta *player.TrackMeta) interface{} {
	if tr == nil {
		return nil
	}
	var struc struct {
		URI         string `json:"uri"`
		Artist      string `json:"artist,omitempty"`
		Title       string `json:"title,omitempty"`
		Genre       string `json:"genre,omitempty"`
		Album       string `json:"album,omitempty"`
		AlbumArtist string `json:"albumartist,omitempty"`
		AlbumTrack  string `json:"albumtrack,omitempty"`
		AlbumDisc   string `json:"albumdisc,omitempty"`
		Duration    int    `json:"duration"`
		HasArt      bool   `json:"hasart"`

		QueuedBy string `json:"queuedby,omitempty"`
	}
	struc.URI = tr.URI
	struc.Artist = tr.Artist
	struc.Title = tr.Title
	struc.Genre = tr.Genre
	struc.Album = tr.Album
	struc.AlbumArtist = tr.AlbumArtist
	struc.AlbumTrack = tr.AlbumTrack
	struc.AlbumDisc = tr.AlbumDisc
	struc.Duration = int(tr.Duration / time.Second)
	struc.HasArt = tr.HasArt
	if meta != nil {
		struc.QueuedBy = meta.QueuedBy
	}
	return struc
}

func trackJSONList(inList []library.Track) (outList []interface{}) {
	outList = make([]interface{}, len(inList))
	for i, tr := range inList {
		outList[i] = trackJSON(&tr, nil)
	}
	return
}

func plTrackJSONList(inList []library.Track, meta []player.TrackMeta, libs []library.Library, trackIndex int) ([]interface{}, error) {
	outList := make([]interface{}, len(inList))
	uris := make([]string, len(inList))
	for i, tr := range inList {
		uris[i] = tr.URI
	}
	tracks, err := library.AllTrackInfo(libs, uris...)
	if err != nil {
		return nil, err
	}

	if trackIndex >= 0 && trackIndex < len(inList) {
		// Because players are allowed to overide the metadata of other sources
		// like the stream database, artwork contained by these secondary
		// sources will be overridden.
		// This is a hacky way to ensure that such artwork will still be served
		// for the current track.
		for _, lib := range libs {
			if image, _ := lib.TrackArt(inList[trackIndex].URI); image != nil {
				image.Close()
				tracks[trackIndex].HasArt = true
				break
			}
		}
	}

	for i, tr := range tracks {
		outList[i] = trackJSON(&tr, &meta[i])
	}
	return outList, nil
}

func htPlayerDataAttach(r *mux.Router, players playerList, streamdb *stream.DB, rawServer *raw.Server, netServer *netmedia.Server) {
	mid := func(handleFunc func(res http.ResponseWriter, req *http.Request)) func(res http.ResponseWriter, req *http.Request) {
		return func(res http.ResponseWriter, req *http.Request) {
			htJSONContent(res, req)

			name := mux.Vars(req)["player"]
			pl := players.ActivePlayerByName(name)
			if pl == nil {
				writeError(req, res, fmt.Errorf("Player %q is not active", name))
				return
			}
			playerCtx := context.WithValue(req.Context(), playerContextKey, pl)

			handleFunc(res, req.WithContext(playerCtx))
		}
	}

	libs := []library.Library{streamdb, rawServer}
	r.Path("/playlist").Methods("GET").HandlerFunc(mid(htPlayerGetPlaylist(libs)))
	r.Path("/playlist").Methods("PUT").HandlerFunc(mid(htPlayerPlaylistInsert()))
	r.Path("/playlist").Methods("PATCH").HandlerFunc(mid(htPlayerPlaylistMove()))
	r.Path("/playlist").Methods("DELETE").HandlerFunc(mid(htPlayerPlaylistRemove()))
	r.Path("/playlist/appendraw").Methods("POST").HandlerFunc(mid(htRawTrackAdd(rawServer)))
	r.Path("/playlist/appendnet").Methods("POST").HandlerFunc(mid(htNetTrackAdd(netServer)))
	r.Path("/current").Methods("POST").HandlerFunc(mid(htPlayerSetCurrent()))
	r.Path("/next").Methods("POST").HandlerFunc(mid(htPlayerNext())) // Deprecated
	r.Path("/time").Methods("GET").HandlerFunc(mid(htPlayerGetTime()))
	r.Path("/time").Methods("POST").HandlerFunc(mid(htPlayerSetTime()))
	r.Path("/playstate").Methods("GET").HandlerFunc(mid(htPlayerGetPlaystate()))
	r.Path("/playstate").Methods("POST").HandlerFunc(mid(htPlayerSetPlaystate()))
	r.Path("/volume").Methods("GET").HandlerFunc(mid(htPlayerGetVolume()))
	r.Path("/volume").Methods("POST").HandlerFunc(mid(htPlayerSetVolume()))
	r.Path("/list/").Methods("GET").HandlerFunc(mid(htPlayerListStoredPlaylists()))
	r.Path("/list/{name}/").Methods("GET").HandlerFunc(mid(htPlayerStoredPlaylistTracks()))
	r.Path("/tracks").Methods("GET").HandlerFunc(mid(htPlayerTracks()))
	r.Path("/tracks/search").Methods("GET").HandlerFunc(mid(htTrackSearch()))
	r.Path("/tracks/art").Methods("GET").HandlerFunc(mid(htTrackArt(libs)))
	r.Path("/listen").HandlerFunc(mid(htPlayerListen()))
}

// Deprecated, use htPlayerSetCurrent instead.
func htPlayerNext() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		trackIndex, err := pl.TrackIndex()
		if err != nil {
			writeError(req, res, err)
			return
		}
		if err := pl.SetTrackIndex(trackIndex + 1); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerSetCurrent() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			Current  int  `json:"current"`
			Relative bool `json:"relative"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		trackIndex := data.Current
		if data.Relative {
			currentTrackIndex, err := pl.TrackIndex()
			if err != nil {
				writeError(req, res, err)
				return
			}
			trackIndex += currentTrackIndex
		}
		if err := pl.SetTrackIndex(trackIndex); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerSetTime() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			Time int `json:"time"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		if err := pl.SetTime(time.Duration(data.Time) * time.Second); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerGetTime() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		tim, err := pl.Time()
		if err != nil {
			writeError(req, res, err)
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"time": int(tim / time.Second),
		})
	}
}

func htPlayerGetPlaystate() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		playstate, err := pl.State()
		if err != nil {
			writeError(req, res, err)
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"playstate": playstate,
		})
	}
}

func htPlayerSetPlaystate() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			State string `json:"playstate"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		if err := pl.SetState(player.PlayState(data.State)); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerGetVolume() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		volume, err := pl.Volume()
		if err != nil {
			writeError(req, res, err)
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"volume": volume,
		})
	}
}

func htPlayerSetVolume() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			Volume float32 `json:"volume"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		if err := pl.SetVolume(data.Volume); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerGetPlaylist(libs []library.Library) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		tracks, err := pl.Playlist().Tracks()
		if err != nil {
			writeError(req, res, err)
			return
		}
		meta, err := pl.Playlist().Meta()
		if err != nil {
			writeError(req, res, err)
			return
		}
		trackIndex, err := pl.TrackIndex()
		if err != nil {
			writeError(req, res, err)
			return
		}
		tim, err := pl.Time()
		if err != nil {
			writeError(req, res, err)
			return
		}
		trJSON, err := plTrackJSONList(tracks, meta, append(libs, pl), trackIndex)
		if err != nil {
			writeError(req, res, err)
			return
		}

		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"time":    int(tim / time.Second),
			"current": trackIndex,
			"tracks":  trJSON,
		})
		if err != nil {
			writeError(req, res, err)
			return
		}
	}
}

func htPlayerPlaylistInsert() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			Pos    int      `json:"position"`
			Tracks []string `json:"tracks"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		tracks := make([]library.Track, len(data.Tracks))
		for i, uri := range data.Tracks {
			tracks[i].URI = uri
		}
		meta := make([]player.TrackMeta, len(data.Tracks))
		for i := range data.Tracks {
			meta[i].QueuedBy = "user"
		}
		plist := pl.Playlist()
		if err := plist.InsertWithMeta(data.Pos, tracks, meta); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerPlaylistMove() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			From int `json:"from"`
			To   int `json:"to"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		if err := pl.Playlist().Move(data.From, data.To); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerPlaylistRemove() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			Positions []int `json:"positions"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		if err := pl.Playlist().Remove(data.Positions...); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htPlayerListStoredPlaylists() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		playlists, err := pl.Lists()
		if err != nil {
			writeError(req, res, err)
			return
		}
		names := make([]string, 0, len(playlists))
		for name := range playlists {
			names = append(names, name)
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"lists": names,
		})
	}
}

func htPlayerStoredPlaylistTracks() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		playlists, err := pl.Lists()
		if err != nil {
			writeError(req, res, err)
			return
		}
		playlist, ok := playlists[mux.Vars(req)["name"]]
		if !ok {
			res.WriteHeader(http.StatusNotFound)
			res.Write([]byte("{}"))
			return
		}
		tracks, err := playlist.Tracks()
		if err != nil {
			writeError(req, res, err)
			return
		}

		outList := make([]interface{}, len(tracks))
		for i, tr := range tracks {
			outList[i] = trackJSON(&tr, nil)
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": outList,
		})
	}
}

func htPlayerTracks() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		tracks, err := pl.Tracks()
		if err != nil {
			writeError(req, res, err)
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": trackJSONList(tracks),
		})
	}
}

func htTrackArt(libs []library.Library) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		uri := req.FormValue("track")
		var image io.ReadCloser
		var mime string
		for _, lib := range append(libs, pl) {
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
		var buf bytes.Buffer
		// Copy to a buffer so seeking is supported.
		io.Copy(&buf, image)
		http.ServeContent(res, req, path.Base(uri), httpCacheSince, bytes.NewReader(buf.Bytes()))
	}
}

func htTrackSearch() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		tracks, err := pl.Tracks()
		if err != nil {
			writeError(req, res, err)
			return
		}

		untaggedFields := strings.Split(req.FormValue("untagged"), ",")
		compiledQuery, err := keyed.CompileQuery(req.FormValue("query"), untaggedFields)
		if err != nil {
			writeError(req, res, err)
			return
		}
		results := filter.Tracks(compiledQuery, tracks)
		sort.Sort(filter.ByNumMatches(results))

		mappedResults := make([]interface{}, len(results))
		for i, res := range results {
			mappedResults[i] = map[string]interface{}{
				"matches": res.Matches,
				"track":   trackJSON(&res.Track, nil),
			}
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": mappedResults,
		})
	}
}

func removeRawTrack(pl player.Player, track library.Track, rawServer *raw.Server) {
	events := pl.Events().Listen()
	defer pl.Events().Unlisten(events)
outer:
	for event := range events {
		if event != "playlist" {
			continue
		}
		tracks, err := pl.Playlist().Tracks()
		if err != nil {
			break
		}
		for _, plTrack := range tracks {
			if track.URI == plTrack.URI {
				continue outer
			}
		}
		break
	}
	rawServer.Remove(track.URI)
}

func htRawTrackAdd(rawServer *raw.Server) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		mpReader, err := req.MultipartReader()
		if err != nil {
			writeError(req, res, err)
			return
		}

		for {
			part, err := mpReader.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				writeError(req, res, err)
				return
			}
			// Make the file available through the server.
			track, errs := rawServer.Add(part, part.FileName(), nil, "")
			if err := <-errs; err != nil {
				writeError(req, res, err)
				return
			}

			// Launch a goroutine that will check whether the track is still in
			// the player's playlist. If it is not, the track is removed from
			// the server.
			go removeRawTrack(pl, track, rawServer)

			err = pl.Playlist().InsertWithMeta(-1, []library.Track{track}, []player.TrackMeta{
				{QueuedBy: "user"},
			})
			if err != nil {
				writeError(req, res, err)
				return
			}
		}
		res.Write([]byte("{}"))
	}
}

func htNetTrackAdd(netServer *netmedia.Server) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		var data struct {
			URL string `json:"url"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		track, errc := netServer.Download(data.URL)
		go func() {
			if err := <-errc; err != nil {
				log.Println(err)
			}
		}()

		// Launch a goroutine that will check whether the track is still in
		// the player's playlist. If it is not, the track is removed from
		// the server.
		go removeRawTrack(pl, track, netServer.RawServer())

		err := pl.Playlist().InsertWithMeta(-1, []library.Track{track}, []player.TrackMeta{
			{QueuedBy: "user"},
		})
		if err != nil {
			writeError(req, res, err)
			return
		}

		res.Write([]byte("{}"))
	}
}

func htPlayerListen() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		pl := req.Context().Value(playerContextKey).(player.Player)
		websocket.Handler(htListen(pl.Events())).ServeHTTP(res, req)
	}
}
