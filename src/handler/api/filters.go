package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"trollibox/src/filter"
	"trollibox/src/util/eventsource"
)

func (api *API) filterList(w http.ResponseWriter, r *http.Request) {
	names, err := api.jukebox.FilterDB().Names()
	if err != nil {
		WriteError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"filters": names,
	})
}

func (api *API) filterGet(w http.ResponseWriter, r *http.Request) {
	filter, err := api.jukebox.FilterDB().Get(chi.URLParam(r, "name"))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if filter == nil {
		// TODO: Return a proper response code.
		WriteError(w, r, fmt.Errorf("not found"))
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"filter": filter,
	})
}

func (api *API) filterRemove(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := api.jukebox.FilterDB().Remove(name); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) filterSet(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Filter json.RawMessage `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	filter, err := filter.UnmarshalJSON(data.Filter)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	name := chi.URLParam(r, "name")
	if err := api.jukebox.FilterDB().Set(name, filter); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) filterEvents(w http.ResponseWriter, r *http.Request) {
	es, err := eventsource.Begin(w, r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	listener := api.jukebox.FilterDB().Listen()
	defer api.jukebox.FilterDB().Unlisten(listener)

	names, err := api.jukebox.FilterDB().Names()
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	for _, name := range names {
		filter, err := api.jukebox.FilterDB().Get(name)
		if err != nil {
			log.Errorf("%v", err)
			return
		}
		es.EventJSON(fmt.Sprintf("filter:%s", name), map[string]interface{}{"filter": filter})
	}

	for {
		var event interface{}
		select {
		case event = <-listener:
		case <-r.Context().Done():
			return
		}

		switch t := event.(type) {
		case filter.UpdateEvent:
			es.EventJSON(fmt.Sprintf("filter:%s", t.Name), map[string]interface{}{"filter": t.Filter})

		default:
			log.Debugf("Unmapped filter db event %#v", event)
		}
	}
}
