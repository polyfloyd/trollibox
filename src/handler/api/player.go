package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"trollibox/src/jukebox"
	"trollibox/src/library"
	"trollibox/src/player"
	"trollibox/src/util/eventsource"
)

var httpCacheSince = time.Now()

type playerContextType struct{}

func jsonTrack(tr *library.Track, meta *player.TrackMeta) interface{} {
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
	if meta != nil {
		struc.QueuedBy = meta.QueuedBy
	}
	return struc
}

func jsonTracks(inList []library.Track) []interface{} {
	outList := make([]interface{}, len(inList))
	for i, tr := range inList {
		outList[i] = jsonTrack(&tr, nil)
	}
	return outList
}

func jsonPlaylistTracks(ctx context.Context, inList []player.MetaTrack, libs []library.Library) ([]interface{}, error) {
	uris := make([]string, len(inList))
	for i, tr := range inList {
		uris[i] = tr.URI
	}
	tracks, err := library.AllTrackInfo(ctx, libs, uris...)
	if err != nil {
		return nil, err
	}

	outList := make([]interface{}, len(inList))
	for i, tr := range tracks {
		outList[i] = jsonTrack(&tr, &inList[i].TrackMeta)
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
	tracks, err := plist.MetaTracks(r.Context())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	trackIndex, err := api.jukebox.PlayerTrackIndex(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	libs, err := api.jukebox.PlayerLibraries(r.Context(), playerName)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	trJSON, err := jsonPlaylistTracks(r.Context(), tracks, libs)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	err = json.NewEncoder(w).Encode(map[string]interface{}{
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
	if err := plist.InsertWithMeta(r.Context(), data.Pos, tracks, meta); err != nil {
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
	if err := plist.Move(r.Context(), data.From, data.To); err != nil {
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
	if err := plist.Remove(r.Context(), data.Positions...); err != nil {
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
	tracks, err := lib.Tracks(r.Context())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tracks": jsonTracks(tracks),
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
		image, mime, err = lib.TrackArt(r.Context(), uri)
		if err == nil {
			break
		}
	}
	if errors.Is(err, library.ErrNoArt) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		WriteError(w, r, err)
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
	untaggedFields := strings.Split(r.FormValue("untagged"), ",")
	results, err := api.jukebox.SearchTracks(r.Context(), playerName, r.FormValue("query"), untaggedFields)
	if errors.Is(err, context.Canceled) {
		return
	} else if err != nil {
		WriteError(w, r, err)
		return
	}

	mappedResults := make([]interface{}, len(results))
	for i, w := range results {
		mappedResults[i] = map[string]interface{}{
			"matches": w.Matches,
			"track":   jsonTrack(&w.Track, nil),
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tracks": mappedResults,
	})
}

func (api *API) playerEvents(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")

	es, err := eventsource.Begin(w, r)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	emitter, err := api.jukebox.PlayerEvents(context.Background(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	listener := emitter.Listen()
	defer emitter.Unlisten(listener)

	libs, err := api.jukebox.PlayerLibraries(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	index, err := api.jukebox.PlayerTrackIndex(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	tracks, err := plist.MetaTracks(r.Context())
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	playlistTracks, err := jsonPlaylistTracks(r.Context(), tracks, libs)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	cTime, err := api.jukebox.PlayerTime(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	es.EventJSON("playlist", map[string]interface{}{"index": index, "tracks": playlistTracks, "time": cTime / time.Second})

	state, err := api.jukebox.PlayerState(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	es.EventJSON("state", map[string]interface{}{"state": state})

	volume, err := api.jukebox.PlayerVolume(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	es.EventJSON("volume", map[string]interface{}{"volume": volume})

	for {
		var event interface{}
		select {
		case event = <-listener:
		case <-r.Context().Done():
			return
		}

		switch t := event.(type) {
		case player.PlaylistEvent:
			tracks, err := plist.MetaTracks(r.Context())
			if err != nil {
				log.Errorf("%v", err)
				return
			}
			playlistTracks, err := jsonPlaylistTracks(r.Context(), tracks, libs)
			if err != nil {
				log.Errorf("%v", err)
				return
			}
			cTime, err := api.jukebox.PlayerTime(r.Context(), playerName)
			if err != nil {
				log.Errorf("%v", err)
				return
			}
			es.EventJSON("playlist", map[string]interface{}{"index": t.Index, "tracks": playlistTracks, "time": cTime / time.Second})
		case player.PlayStateEvent:
			es.EventJSON("state", map[string]interface{}{"state": t.State})
		case player.TimeEvent:
			es.EventJSON("time", map[string]interface{}{"time": int(t.Time / time.Second)})
		case player.VolumeEvent:
			es.EventJSON("volume", map[string]interface{}{"volume": t.Volume})
		case player.AvailabilityEvent:
			es.EventJSON("availability", map[string]interface{}{"available": t.Available})
		case library.UpdateEvent:
			es.EventJSON("library", "")
		default:
			log.Debugf("Unmapped filter db event %#v", event)
		}
	}
}
