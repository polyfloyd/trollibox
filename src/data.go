package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"./filter"
	"./filter/keyed"
	"./filter/ruled"
	"./player"
	"./stream"
	"./util"
	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)

func htDataAttach(r *mux.Router, filterdb *filter.DB, streamdb *stream.DB, rawServer *player.RawTrackServer) {
	r.Path("/filters/").Methods("GET").HandlerFunc(htFilterList(filterdb))
	r.Path("/filters/{name}/").Methods("GET").HandlerFunc(htFilterGet(filterdb))
	r.Path("/filters/{name}/").Methods("DELETE").HandlerFunc(htFilterRemove(filterdb))
	r.Path("/filters/{name}/").Methods("PUT").HandlerFunc(htFilterSet(filterdb))
	r.Path("/filters/listen").Handler(websocket.Handler(htListen(&filterdb.Emitter)))
	r.Path("/streams").Methods("GET").HandlerFunc(htStreamsList(streamdb))
	r.Path("/streams").Methods("POST").HandlerFunc(htStreamsAdd(streamdb))
	r.Path("/streams").Methods("DELETE").HandlerFunc(htStreamsRemove(streamdb))
	r.Path("/streams/listen").Handler(websocket.Handler(websocket.Handler(htListen(&streamdb.Emitter))))
	r.Path("/raw").Methods("GET").Handler(rawServer)
}

// Writes an error to the client or an empty object if err is nil.
func writeError(res http.ResponseWriter, err error) {
	log.Printf("Error serving: %v, %v", err)
	res.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(res).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func htListen(emitter *util.Emitter) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		ch := emitter.Listen()
		defer emitter.Unlisten(ch)
		conn.SetDeadline(time.Time{})
		for {
			if _, err := conn.Write([]uint8(<-ch)); err != nil {
				break
			}
		}
	}
}

func htFilterList(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		filters := filterdb.Filters()
		names := make([]string, 0, len(filters))
		for name := range filters {
			names = append(names, name)
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"filters": names,
		})
	}
}

func htFilterGet(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		filters := filterdb.Filters()
		filter, ok := filters[mux.Vars(req)["name"]]
		if !ok {
			// TODO: Return a proper response code.
			writeError(res, fmt.Errorf("Not found"))
			return
		}

		typ := ""
		switch filter.(type) {
		case *ruled.RuleFilter:
			typ = "ruled"
		case *keyed.Query:
			typ = "keyed"
		default:
			writeError(res, fmt.Errorf("Unknown filter type %T", filter))
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"filter": map[string]interface{}{
				"type":  typ,
				"value": filter,
			},
		})
	}
}

func htFilterRemove(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		name := mux.Vars(req)["name"]
		if err := filterdb.Remove(name); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htFilterSet(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Filter struct {
				Type  string          `json:"type"`
				Value json.RawMessage `json:"value"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		var filter filter.Filter
		switch data.Filter.Type {
		case "ruled":
			filter = &ruled.RuleFilter{}
		case "keyed":
			filter = &keyed.Query{}
		default:
			writeError(res, fmt.Errorf("Unknown filter type %q", data.Filter.Type))
			return
		}

		if err := json.Unmarshal([]byte(data.Filter.Value), filter); err != nil {
			writeError(res, err)
			return
		}
		name := mux.Vars(req)["name"]
		if err := filterdb.Set(name, filter); err != nil {
			writeError(res, err)
			return
		}
		// TODO: Handle RuleError
		res.Write([]byte("{}"))
	}
}

func htStreamsList(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		streams := streamdb.Streams()
		mapped := make([]interface{}, len(streams))
		for i, stream := range streams {
			mapped[i] = map[string]interface{}{
				"uri":    stream.Url,
				"title":  stream.StreamTitle,
				"hasart": stream.ArtUrl != "",
			}
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"streams": mapped,
		})
	}
}

func htStreamsAdd(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Stream stream.Stream `json:"stream"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := streamdb.AddStream(data.Stream); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htStreamsRemove(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		uri := req.FormValue("uri")
		if err := streamdb.RemoveStreamByUrl(uri); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}
