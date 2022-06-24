package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"trollibox/src/filter"
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
	listener := api.jukebox.FilterDB().Listen()
	defer api.jukebox.FilterDB().Unlisten(listener)

	names, err := api.jukebox.FilterDB().Names()
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	es.EventJSON("list", map[string]interface{}{"filters": names})
	for _, name := range names {
		filter, err := api.jukebox.FilterDB().Get(name)
		if err != nil {
			log.Errorf("%v", err)
			return
		}
		es.EventJSON("update", map[string]interface{}{
			"name":   name,
			"filter": filter,
		})
	}

	for {
		var event interface{}
		select {
		case event = <-listener:
		case <-r.Context().Done():
			return
		}

		switch t := event.(type) {
		case filter.ListEvent:
			es.EventJSON("list", map[string]interface{}{"filters": t.Names})
		case filter.UpdateEvent:
			es.EventJSON("update", map[string]interface{}{
				"name":   t.Name,
				"filter": t.Filter,
			})

		default:
			log.Debugf("Unmapped filter db event %#v", event)
		}
	}
}
