package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi"

	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/keyed"
	"github.com/polyfloyd/trollibox/src/jukebox"
	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/player"
)

var httpCacheSince = time.Now()

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

// API contains the state that is accessible over the Trollibox REST API.
type API struct {
	jukebox *jukebox.Jukebox
}

// Deprecated, use setCurrent instead.
func (api *API) playerNext(w http.ResponseWriter, r *http.Request) {
	if err := api.jukebox.SetPlayerTrackIndex(r.Context(), chi.URLParam(r, "playerName"), 1, true); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playerSetCurrent(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Current  int  `json:"current"`
		Relative bool `json:"relative"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	if err := api.jukebox.SetPlayerTrackIndex(r.Context(), chi.URLParam(r, "playerName"), data.Current, data.Relative); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playerSetTime(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Time int `json:"time"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	if err := api.jukebox.SetPlayerTime(r.Context(), chi.URLParam(r, "playerName"), time.Duration(data.Time)*time.Second); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playerGetTime(w http.ResponseWriter, r *http.Request) {
	tim, err := api.jukebox.PlayerTime(r.Context(), chi.URLParam(r, "playerName"))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"time": int(tim / time.Second),
	})
}

func (api *API) playerGetPlaystate(w http.ResponseWriter, r *http.Request) {
	playstate, err := api.jukebox.PlayerState(r.Context(), chi.URLParam(r, "playerName"))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"playstate": playstate,
	})
}

func (api *API) playerSetPlaystate(w http.ResponseWriter, r *http.Request) {
	var data struct {
		State string `json:"playstate"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	if err := api.jukebox.SetPlayerState(r.Context(), chi.URLParam(r, "playerName"), player.PlayState(data.State)); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playerGetVolume(w http.ResponseWriter, r *http.Request) {
	volume, err := api.jukebox.PlayerVolume(r.Context(), chi.URLParam(r, "playerName"))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"volume": float32(volume) / 100.0,
	})
}

func (api *API) playerSetVolume(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Volume float32 `json:"volume"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	if err := api.jukebox.SetPlayerVolume(r.Context(), chi.URLParam(r, "playerName"), int(data.Volume*100)); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playlistContents(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	tracks, err := plist.Tracks()
	if err != nil {
		WriteError(w, r, err)
		return
	}
	meta, err := plist.Meta()
	if err != nil {
		WriteError(w, r, err)
		return
	}
	trackIndex, err := api.jukebox.PlayerTrackIndex(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	tim, err := api.jukebox.PlayerTime(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	libs, err := api.jukebox.PlayerLibraries(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	trJSON, err := plTrackJSONList(tracks, meta, libs, trackIndex)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"time":    int(tim / time.Second),
		"current": trackIndex,
		"tracks":  trJSON,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}
}

func (api *API) playlistInsert(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	var data struct {
		Pos    int      `json:"position"`
		Tracks []string `json:"tracks"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
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
	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if err := plist.InsertWithMeta(data.Pos, tracks, meta); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playlistMove(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	var data struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if err := plist.Move(data.From, data.To); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playlistRemove(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	var data struct {
		Positions []int `json:"positions"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if err := plist.Remove(data.Positions...); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) playerTracks(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	lib, err := api.jukebox.PlayerLibrary(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	tracks, err := lib.Tracks()
	if err != nil {
		WriteError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tracks": trackJSONList(tracks),
	})
}

func (api *API) playerTrackArt(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	uri := r.FormValue("track")

	libs, err := api.jukebox.PlayerLibraries(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	var image io.ReadCloser
	var mime string
	for _, lib := range libs {
		if image, mime = lib.TrackArt(uri); image != nil {
			break
		}
	}
	if image == nil {
		http.NotFound(w, r)
		return
	}
	defer image.Close()

	w.Header().Set("Content-Type", mime)
	var buf bytes.Buffer
	// Copy to a buffer so seeking is supported.
	io.Copy(&buf, image)
	http.ServeContent(w, r, path.Base(uri), httpCacheSince, bytes.NewReader(buf.Bytes()))
}

func (api *API) playerTrackSearch(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	lib, err := api.jukebox.PlayerLibrary(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	tracks, err := lib.Tracks()
	if err != nil {
		WriteError(w, r, err)
		return
	}

	untaggedFields := strings.Split(r.FormValue("untagged"), ",")
	compiledQuery, err := keyed.CompileQuery(r.FormValue("query"), untaggedFields)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	wults := filter.Tracks(compiledQuery, tracks)
	sort.Sort(filter.ByNumMatches(wults))

	mappedResults := make([]interface{}, len(wults))
	for i, w := range wults {
		mappedResults[i] = map[string]interface{}{
			"matches": w.Matches,
			"track":   trackJSON(&w.Track, nil),
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tracks": mappedResults,
	})
}

func (api *API) playerEvents() http.Handler {
	var eventSourcesLock sync.Mutex
	eventSources := map[string]http.Handler{}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		playerName := chi.URLParam(r, "playerName")

		eventSourcesLock.Lock()
		ev, ok := eventSources[playerName]
		if !ok {
			emitter, err := api.jukebox.PlayerEvents(context.Background(), playerName)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			ev = htEvents(emitter)
			eventSources[playerName] = ev
		}
		eventSourcesLock.Unlock()

		ev.ServeHTTP(w, r)
	})
}
