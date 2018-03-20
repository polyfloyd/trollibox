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

type filterAPI struct {
	db *filter.DB
}

func (api *filterAPI) list(res http.ResponseWriter, req *http.Request) {
	names, err := api.db.Names()
	if err != nil {
		WriteError(req, res, err)
		return
	}
	json.NewEncoder(res).Encode(map[string]interface{}{
		"filters": names,
	})
}

func (api *filterAPI) get(res http.ResponseWriter, req *http.Request) {
	filter, err := api.db.Get(chi.URLParam(req, "name"))
	if err != nil {
		WriteError(req, res, err)
		return
	}
	if filter == nil {
		// TODO: Return a proper response code.
		WriteError(req, res, fmt.Errorf("not found"))
		return
	}

	typ := ""
	switch filter.(type) {
	case *ruled.RuleFilter:
		typ = "ruled"
	case *keyed.Query:
		typ = "keyed"
	default:
		WriteError(req, res, fmt.Errorf("unknown filter type %T", filter))
		return
	}
	json.NewEncoder(res).Encode(map[string]interface{}{
		"filter": map[string]interface{}{
			"type":  typ,
			"value": filter,
		},
	})
}

func (api *filterAPI) remove(res http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	if err := api.db.Remove(name); err != nil {
		WriteError(req, res, err)
		return
	}
	res.Write([]byte("{}"))
}

func (api *filterAPI) set(res http.ResponseWriter, req *http.Request) {
	var data struct {
		Filter struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"filter"`
	}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		WriteError(req, res, err)
		return
	}

	var filter filter.Filter
	switch data.Filter.Type {
	case "ruled":
		filter = &ruled.RuleFilter{}
	case "keyed":
		filter = &keyed.Query{}
	default:
		WriteError(req, res, fmt.Errorf("unknown filter type %q", data.Filter.Type))
		return
	}

	if err := json.Unmarshal([]byte(data.Filter.Value), filter); err != nil {
		WriteError(req, res, err)
		return
	}
	name := chi.URLParam(req, "name")
	if err := api.db.Set(name, filter); err != nil {
		WriteError(req, res, err)
		return
	}
	res.Write([]byte("{}"))
}
