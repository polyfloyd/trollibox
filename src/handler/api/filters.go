package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"trollibox/src/filter"
	"trollibox/src/jukebox"
	"trollibox/src/util/eventsource"
)

func (api *API) filterList(w http.ResponseWriter, r *http.Request) {
	names, err := api.jukebox.FilterDB().Names()
	if api.mapError(w, r, err) {
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"filters": names,
	})
}

func (api *API) filterGet(w http.ResponseWriter, r *http.Request) {
	filter, err := api.jukebox.FilterDB().Get(chi.URLParam(r, "name"))
	if api.mapError(w, r, err) {
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"filter": filter,
	})
}

func (api *API) filterRemove(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := api.jukebox.FilterDB().Remove(name); api.mapError(w, r, err) {
		return
	}

	_, _ = w.Write([]byte("{}"))
}

func (api *API) filterSet(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Filter json.RawMessage `json:"filter"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}
	filter, err := filter.UnmarshalJSON(data.Filter)
	if api.mapError(w, r, err) {
		return
	}

	name := chi.URLParam(r, "name")
	if err := api.jukebox.FilterDB().Set(name, filter); api.mapError(w, r, err) {
		return
	}

	_, _ = w.Write([]byte("{}"))
}

func (api *API) filterEvents(w http.ResponseWriter, r *http.Request) {
	es, err := eventsource.Begin(w, r)
	if api.mapError(w, r, err) {
		return
	}
	filterListener := api.jukebox.FilterDB().Listen(r.Context())
	jukeboxListener := api.jukebox.Listen(r.Context())

	names, err := api.jukebox.FilterDB().Names()
	if err != nil {
		slog.Error("Could not list filter names", "error", err)
		return
	}
	es.EventJSON("list", map[string]interface{}{"filters": names})
	for _, name := range names {
		filter, err := api.jukebox.FilterDB().Get(name)
		if err != nil {
			slog.Error("Could get filter", "error", err, "name", name)
			return
		}
		es.EventJSON("update", map[string]interface{}{
			"name":   name,
			"filter": filter,
		})
	}
	playerFilters := api.jukebox.PlayerAutoQueuerFilters(r.Context())
	for playerName, filterName := range playerFilters {
		es.EventJSON("autoqueuer", map[string]interface{}{
			"player": playerName,
			"filter": filterName,
		})
	}

	for {
		select {
		case event := <-filterListener:
			switch t := event.(type) {
			case filter.ListEvent:
				es.EventJSON("list", map[string]interface{}{"filters": t.Names})
			case filter.UpdateEvent:
				es.EventJSON("update", map[string]interface{}{
					"name":   t.Name,
					"filter": t.Filter,
				})
			default:
				slog.Debug("Unmapped filter db event", "event", event)
			}

		case event := <-jukeboxListener:
			switch t := event.(type) {
			case jukebox.PlayerAutoQueuerEvent:
				es.EventJSON("autoqueuer", map[string]interface{}{
					"player": t.PlayerName,
					"filter": t.FilterName,
				})
			default:
				slog.Debug("Unmapped jukebox event", "event", event)
			}

		case <-r.Context().Done():
			return
		}
	}
}
