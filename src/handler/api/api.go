package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"trollibox/src/jukebox"
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
		r.Get("/events", api.playerEvents)
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

func jsonCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
