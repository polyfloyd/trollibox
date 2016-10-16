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
	raw "./library/raw"
	"./library/stream"
	"./util"
	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)

func htDataAttach(r *mux.Router, filterdb *filter.DB, streamdb *stream.DB, rawServer *raw.Server) {
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
func writeError(req *http.Request, res http.ResponseWriter, err error) {
	log.Printf("Error serving %s: %v", req.RemoteAddr, err)
	res.WriteHeader(http.StatusBadRequest)

	if req.Header.Get("X-Requested-With") == "" {
		res.Write([]byte(err.Error()))
		return
	}

	data, _ := json.Marshal(err)
	if data == nil {
		data = []byte("{}")
	}
	json.NewEncoder(res).Encode(map[string]interface{}{
		"error": err.Error(),
		"data":  (*json.RawMessage)(&data),
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
		hmJsonContent(res, req)
		names, err := filterdb.Names()
		if err != nil {
			writeError(req, res, err)
			return
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"filters": names,
		})
	}
}

func htFilterGet(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		hmJsonContent(res, req)
		filter, err := filterdb.Get(mux.Vars(req)["name"])
		if err != nil {
			writeError(req, res, err)
			return
		}
		if filter == nil {
			// TODO: Return a proper response code.
			writeError(req, res, fmt.Errorf("Not found"))
			return
		}

		typ := ""
		switch filter.(type) {
		case *ruled.RuleFilter:
			typ = "ruled"
		case *keyed.Query:
			typ = "keyed"
		default:
			writeError(req, res, fmt.Errorf("Unknown filter type %T", filter))
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
		hmJsonContent(res, req)
		name := mux.Vars(req)["name"]
		if err := filterdb.Remove(name); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htFilterSet(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		hmJsonContent(res, req)
		var data struct {
			Filter struct {
				Type  string          `json:"type"`
				Value json.RawMessage `json:"value"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		var filter filter.Filter
		switch data.Filter.Type {
		case "ruled":
			filter = &ruled.RuleFilter{}
		case "keyed":
			filter = &keyed.Query{}
		default:
			writeError(req, res, fmt.Errorf("Unknown filter type %q", data.Filter.Type))
			return
		}

		if err := json.Unmarshal([]byte(data.Filter.Value), filter); err != nil {
			writeError(req, res, err)
			return
		}
		name := mux.Vars(req)["name"]
		if err := filterdb.Store(name, filter); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htStreamsList(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		hmJsonContent(res, req)
		streams, err := streamdb.Streams()
		if err != nil {
			writeError(req, res, err)
			return
		}
		mapped := make([]interface{}, len(streams))
		for i, stream := range streams {
			mapped[i] = map[string]interface{}{
				"filename": stream.Filename,
				"url":      stream.URL,
				"title":    stream.Title,
				"hasart":   stream.ArtURI != "",
			}
		}
		json.NewEncoder(res).Encode(map[string]interface{}{
			"streams": mapped,
		})
	}
}

func htStreamsAdd(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		hmJsonContent(res, req)
		var data struct {
			Stream stream.Stream `json:"stream"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(req, res, err)
			return
		}

		if data.Stream.ArtURI == "" && data.Stream.Filename != "" {
			// Retain the artwork if no new uri is provided.
			tmpl, err := streamdb.StreamByFilename(data.Stream.Filename)
			if err != nil {
				writeError(req, res, err)
				return
			}
			data.Stream.ArtURI = tmpl.ArtURI
		}

		if err := streamdb.StoreStream(&data.Stream); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htStreamsRemove(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		hmJsonContent(res, req)
		stream := stream.Stream{Filename: req.FormValue("filename")}
		if err := streamdb.RemoveStream(&stream); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}
