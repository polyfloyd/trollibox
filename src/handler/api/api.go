package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/antage/eventsource"
	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"trollibox/src/filter"
	"trollibox/src/jukebox"
	"trollibox/src/library"
	"trollibox/src/player"
	"trollibox/src/util"
)

// InitRouter attaches all API routes to the specified router.
func InitRouter(r chi.Router, jukebox *jukebox.Jukebox) {
	api := API{jukebox: jukebox}
	r.Route("/player/{playerName}", func(r chi.Router) {
		r.Use(jsonCtx)
		r.Route("/playlist", func(r chi.Router) {
			r.Get("/", api.playlistContents)
			r.Put("/", api.playlistInsert)
			r.Patch("/", api.playlistMove)
			r.Delete("/", api.playlistRemove)
		})
		r.Post("/current", api.playerSetCurrent)
		r.Post("/next", api.playerNext) // Deprecated
		r.Get("/time", api.playerGetTime)
		r.Post("/time", api.playerSetTime)
		r.Get("/playstate", api.playerGetPlaystate)
		r.Post("/playstate", api.playerSetPlaystate)
		r.Get("/volume", api.playerGetVolume)
		r.Post("/volume", api.playerSetVolume)
		r.Get("/tracks", api.playerTracks)
		r.Get("/tracks/search", api.playerTrackSearch)
		r.Get("/tracks/art", api.playerTrackArt)
		r.Mount("/events", api.playerEvents())
	})

	r.Route("/filters/", func(r chi.Router) {
		r.Get("/", api.filterList)
		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", api.filterGet)
			r.Delete("/", api.filterRemove)
			r.Put("/", api.filterSet)
		})
		r.Get("/events", api.filterEvents)
	})

	r.Route("/streams", func(r chi.Router) {
		r.Get("/", api.streamsList)
		r.Post("/", api.streamsAdd)
		r.Delete("/", api.streamsRemove)
		r.Get("/events", api.streamEvents)
	})
}

// WriteError writes an error to the client or an empty object if err is nil.
//
// An attempt is made to tune the response format to the requestor.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	log.Errorf("Error serving %s: %v", r.RemoteAddr, err)
	w.WriteHeader(http.StatusBadRequest)

	if r.Header.Get("X-Requested-With") == "" {
		w.Write([]byte(err.Error()))
		return
	}

	data, _ := json.Marshal(err)
	if data == nil {
		data = []byte("{}")
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
		"data":  (*json.RawMessage)(&data),
	})
}

func htEvents(emitter *util.Emitter) http.Handler {
	conf := eventsource.DefaultSettings()
	events := eventsource.New(conf, func(r *http.Request) [][]byte {
		return [][]byte{
			[]byte("X-Accel-Buffering: no"),
		}
	})

	ch := emitter.Listen()
	go func() {
		id := 0
		for event := range ch {
			id++

			// TODO: All these events should not all be combined in here.
			var eventStr string
			var eventObj interface{}
			switch t := event.(type) {
			case player.PlaylistEvent:
				eventStr, eventObj = "playlist", map[string]interface{}{
					"index": t.Index,
				}
			case player.PlayStateEvent:
				eventStr, eventObj = "playstate", map[string]interface{}{
					"state": t.State,
				}
			case player.TimeEvent:
				eventStr, eventObj = "time", map[string]interface{}{
					"time": int(t.Time / time.Second),
				}
			case player.VolumeEvent:
				eventStr, eventObj = "volume", map[string]interface{}{
					"volume": float32(t.Volume) / 100.0,
				}
			case player.ListEvent:
				eventStr, eventObj = "list", struct{}{}
			case player.AvailabilityEvent:
				eventStr, eventObj = "availability", map[string]interface{}{
					"available": t.Available,
				}
			case library.UpdateEvent:
				eventStr, eventObj = "library:tracks", struct{}{}
			case filter.UpdateEvent:
				eventStr, eventObj = "filter:update", map[string]interface{}{
					"filter": t.Filter,
				}
			default:
				log.Debugf("Unmapped event %#v", event)
				continue
			}

			eventMsg, err := json.Marshal(eventObj)
			if err != nil {
				log.Error(err)
				continue
			}
			events.SendEventMessage(string(eventMsg), eventStr, fmt.Sprintf("%d", id))
		}
	}()
	return events
}

func jsonCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
