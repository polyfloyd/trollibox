package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"trollibox/src/filter"
	"trollibox/src/jukebox"
)

// InitRouter attaches all API routes to the specified router.
func InitRouter(r chi.Router, jukebox *jukebox.Jukebox) {
	api := API{jukebox: jukebox}
	r.Use(jsonCtx)
	r.Route("/player/{playerName}", func(r chi.Router) {
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
		r.Post("/autoqueuer", api.playerSetAutoQueuer)
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

func (api *API) mapError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}

	status := http.StatusInternalServerError
	if errors.Is(err, filter.ErrNotFound) {
		status = http.StatusNotFound
	}

	respondError(w, r, status, err)
	return true
}

func respondError(w http.ResponseWriter, r *http.Request, status int, err error) {
	w.WriteHeader(status)

	if status >= 500 {
		slog.Error("API error", "addr", r.RemoteAddr, "error", err)
	} else {
		slog.Warn("API error", "addr", r.RemoteAddr, "error", err)
	}

	data, _ := json.Marshal(err)
	if data == nil {
		data = []byte("{}")
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
		"data":  (*json.RawMessage)(&data),
	})
}

func receiveJSONForm[T any](w http.ResponseWriter, r *http.Request, recv *T) bool {
	if err := json.NewDecoder(r.Body).Decode(recv); err != nil {
		respondError(w, r, http.StatusBadRequest, err)
		return true
	}
	return false
}

func jsonCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
