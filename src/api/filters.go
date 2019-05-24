package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/keyed"
	"github.com/polyfloyd/trollibox/src/filter/ruled"
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

	typ := ""
	switch filter.(type) {
	case *ruled.RuleFilter:
		typ = "ruled"
	case *keyed.Query:
		typ = "keyed"
	default:
		WriteError(w, r, fmt.Errorf("unknown filter type %T", filter))
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"filter": map[string]interface{}{
			"type":  typ,
			"value": filter,
		},
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
		Filter struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	var filter filter.Filter
	switch data.Filter.Type {
	case "ruled":
		filter = &ruled.RuleFilter{}
	case "keyed":
		filter = &keyed.Query{}
	default:
		WriteError(w, r, fmt.Errorf("unknown filter type %q", data.Filter.Type))
		return
	}

	if err := json.Unmarshal([]byte(data.Filter.Value), filter); err != nil {
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
