package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type rawJsonTrack struct {
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

func jsonTrack(tr *library.Track) *rawJsonTrack {
	if tr == nil {
		return nil
	}
	return &rawJsonTrack{
		URI:         tr.URI,
		Artist:      tr.Artist,
		Title:       tr.Title,
		Genre:       tr.Genre,
		Album:       tr.Album,
		AlbumArtist: tr.AlbumArtist,
		AlbumTrack:  tr.AlbumTrack,
		AlbumDisc:   tr.AlbumDisc,
		Duration:    int(tr.Duration / time.Second),
	}
}

func jsonMetaTrack(tr *player.MetaTrack) *rawJsonTrack {
	jt := jsonTrack(&tr.Track)
	if jt == nil {
		return nil
	}
	jt.QueuedBy = tr.QueuedBy
	return jt
}

func jsonTracks(inList []library.Track) []interface{} {
	outList := make([]interface{}, len(inList))
	for i, tr := range inList {
		outList[i] = jsonTrack(&tr)
	}
	return outList
}

func jsonPlaylistTracks(tracks []player.MetaTrack) ([]interface{}, error) {
	outList := make([]interface{}, len(tracks))
	for i, tr := range tracks {
		outList[i] = jsonMetaTrack(&tr)
	}
	return outList, nil
}

// API contains the state that is accessible over the Trollibox REST API.
type API struct {
	jukebox *jukebox.Jukebox
}

// Deprecated, use setCurrent instead.
func (api *API) playerNext(w http.ResponseWriter, r *http.Request) {
	if err := api.jukebox.SetPlayerTrackIndex(r.Context(), chi.URLParam(r, "playerName"), 1, true); api.mapError(w, r, err) {
		return
	}
	_, _ = w.Write([]byte("{}"))
}

func (api *API) playerSetCurrent(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Current  int  `json:"current"`
		Relative bool `json:"relative"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	if err := api.jukebox.SetPlayerTrackIndex(r.Context(), chi.URLParam(r, "playerName"), data.Current, data.Relative); api.mapError(w, r, err) {
		return
	}
	_, _ = w.Write([]byte("{}"))
}

func (api *API) playerSetTime(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Time int `json:"time"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	if err := api.jukebox.SetPlayerTime(r.Context(), chi.URLParam(r, "playerName"), time.Duration(data.Time)*time.Second); api.mapError(w, r, err) {
		return
	}
	_, _ = w.Write([]byte("{}"))
}

func (api *API) playerGetTime(w http.ResponseWriter, r *http.Request) {
	status, err := api.jukebox.PlayerStatus(r.Context(), chi.URLParam(r, "playerName"))
	if api.mapError(w, r, err) {
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"time": int(status.Time / time.Second),
	})
}

func (api *API) playerGetPlaystate(w http.ResponseWriter, r *http.Request) {
	status, err := api.jukebox.PlayerStatus(r.Context(), chi.URLParam(r, "playerName"))
	if api.mapError(w, r, err) {
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"playstate": status.PlayState,
	})
}

func (api *API) playerSetPlaystate(w http.ResponseWriter, r *http.Request) {
	var data struct {
		State string `json:"playstate"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	if err := api.jukebox.SetPlayerState(r.Context(), chi.URLParam(r, "playerName"), player.PlayState(data.State)); api.mapError(w, r, err) {
		return
	}
	_, _ = w.Write([]byte("{}"))
}

func (api *API) playerGetVolume(w http.ResponseWriter, r *http.Request) {
	status, err := api.jukebox.PlayerStatus(r.Context(), chi.URLParam(r, "playerName"))
	if api.mapError(w, r, err) {
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"volume": float32(status.Volume) / 100.0,
	})
}

func (api *API) playerSetVolume(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Volume float32 `json:"volume"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	if err := api.jukebox.SetPlayerVolume(r.Context(), chi.URLParam(r, "playerName"), int(data.Volume*100)); api.mapError(w, r, err) {
		return
	}
	_, _ = w.Write([]byte("{}"))
}

func (api *API) playlistContents(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if api.mapError(w, r, err) {
		return
	}
	tracks, err := plist.Tracks(r.Context())
	if api.mapError(w, r, err) {
		return
	}
	status, err := api.jukebox.PlayerStatus(r.Context(), playerName)
	if api.mapError(w, r, err) {
		return
	}
	trJSON, err := jsonPlaylistTracks(tracks)
	if api.mapError(w, r, err) {
		return
	}

	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"current": status.TrackIndex,
		"tracks":  trJSON,
	})
	if api.mapError(w, r, err) {
		return
	}
}

func (api *API) playlistInsert(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	var data struct {
		At     string   `json:"at"`
		Pos    int      `json:"position"`
		Tracks []string `json:"tracks"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	tracks := make([]player.MetaTrack, len(data.Tracks))
	for i, uri := range data.Tracks {
		tracks[i].URI = uri
		tracks[i].QueuedBy = "user"
	}
	if err := api.jukebox.PlayerPlaylistInsertAt(r.Context(), playerName, data.At, data.Pos, tracks); api.mapError(w, r, err) {
		return
	}
	_, _ = w.Write([]byte("{}"))
}

func (api *API) playlistMove(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	var data struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if api.mapError(w, r, err) {
		return
	}
	if err := plist.Move(r.Context(), data.From, data.To); api.mapError(w, r, err) {
		return
	}

	_, _ = w.Write([]byte("{}"))
}

func (api *API) playlistRemove(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	var data struct {
		Positions []int `json:"positions"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if api.mapError(w, r, err) {
		return
	}
	if err := plist.Remove(r.Context(), data.Positions...); api.mapError(w, r, err) {
		return
	}

	_, _ = w.Write([]byte("{}"))
}

func (api *API) playerTracks(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	tracks, err := api.jukebox.Tracks(r.Context(), playerName)
	if api.mapError(w, r, err) {
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tracks": jsonTracks(tracks),
	})
}

func (api *API) playerTrackArt(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	uri := r.FormValue("track")

	art, err := api.jukebox.TrackArt(r.Context(), playerName, uri)
	if errors.Is(err, library.ErrNoArt) {
		http.NotFound(w, r)
		return
	} else if api.mapError(w, r, err) {
		return
	}

	w.Header().Set("Content-Type", art.MimeType)
	http.ServeContent(w, r, path.Base(uri), art.ModTime, bytes.NewReader(art.ImageData))
}

func (api *API) playerTrackSearch(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")
	untaggedFields := strings.Split(r.FormValue("untagged"), ",")
	results, err := api.jukebox.SearchTracks(r.Context(), playerName, r.FormValue("query"), untaggedFields)
	if api.mapError(w, r, err) {
		return
	}

	mappedResults := make([]interface{}, len(results))
	for i, w := range results {
		mappedResults[i] = map[string]interface{}{
			"matches": w.Matches,
			"track":   jsonTrack(&w.Track),
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tracks": mappedResults,
	})
}

func (api *API) playerEvents(w http.ResponseWriter, r *http.Request) {
	playerName := chi.URLParam(r, "playerName")

	es, err := eventsource.Begin(w, r)
	if api.mapError(w, r, err) {
		return
	}
	emitter, err := api.jukebox.PlayerEvents(context.Background(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	listener := emitter.Listen(r.Context())

	plist, err := api.jukebox.PlayerPlaylist(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	status, err := api.jukebox.PlayerStatus(r.Context(), playerName)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	tracks, err := plist.Tracks(r.Context())
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	playlistTracks, err := jsonPlaylistTracks(tracks)
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	es.EventJSON("playlist", map[string]interface{}{"index": status.TrackIndex, "tracks": playlistTracks, "time": status.Time / time.Second})
	es.EventJSON("state", map[string]interface{}{"state": status.PlayState})
	es.EventJSON("volume", map[string]interface{}{"volume": status.Volume})

	for event := range listener {
		switch t := event.(type) {
		case player.PlaylistEvent:
			tracks, err := plist.Tracks(r.Context())
			if err != nil {
				log.Errorf("%v", err)
				return
			}
			playlistTracks, err := jsonPlaylistTracks(tracks)
			if err != nil {
				log.Errorf("%v", err)
				return
			}
			status, err := api.jukebox.PlayerStatus(r.Context(), playerName)
			if err != nil {
				log.Errorf("%v", err)
				return
			}
			es.EventJSON("playlist", map[string]interface{}{"index": t.TrackIndex, "tracks": playlistTracks, "time": status.Time / time.Second})
		case player.PlayStateEvent:
			es.EventJSON("state", map[string]interface{}{"state": t.State})
		case player.TimeEvent:
			es.EventJSON("time", map[string]interface{}{"time": int(t.Time / time.Second)})
		case player.VolumeEvent:
			es.EventJSON("volume", map[string]interface{}{"volume": t.Volume})
		case library.UpdateEvent:
			es.EventJSON("library", "")
		default:
			log.Debugf("Unmapped filter db event %#v", event)
		}
	}
}
