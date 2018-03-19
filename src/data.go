package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"golang.org/x/net/websocket"

	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/keyed"
	"github.com/polyfloyd/trollibox/src/filter/ruled"
	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/library/raw"
	"github.com/polyfloyd/trollibox/src/library/stream"
	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/util"
)

func htDataAttach(r chi.Router, filterdb *filter.DB, streamdb *stream.DB, rawServer *raw.Server) {
	r.Route("/filters/", func(r chi.Router) {
		r.Get("/", htFilterList(filterdb))
		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", htFilterGet(filterdb))
			r.Delete("/", htFilterRemove(filterdb))
			r.Put("/", htFilterSet(filterdb))
		})
		r.Mount("/listen", websocket.Handler(htListen(&filterdb.Emitter)))
	})
	r.Route("/streams", func(r chi.Router) {
		r.Get("/", htStreamsList(streamdb))
		r.Post("/", htStreamsAdd(streamdb))
		r.Delete("/", htStreamsRemove(streamdb))
		r.Mount("/listen", websocket.Handler(htListen(&streamdb.Emitter)))
	})
	r.Mount("/raw", rawServer)
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
		// TODO: All these events should not all be combined in here.
		for event := range ch {
			var eventStr string
			switch t := event.(type) {
			case player.Event:
				eventStr = string(t)
			case library.UpdateEvent:
				eventStr = "tracks"
			case filter.UpdateEvent:
				eventStr = "update"
			default:
				continue
			}
			if _, err := conn.Write([]uint8(eventStr)); err != nil {
				break
			}
		}
	}
}

func htFilterList(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		htJSONContent(res, req)
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
		htJSONContent(res, req)
		filter, err := filterdb.Get(chi.URLParam(req, "name"))
		if err != nil {
			writeError(req, res, err)
			return
		}
		if filter == nil {
			// TODO: Return a proper response code.
			writeError(req, res, fmt.Errorf("not found"))
			return
		}

		typ := ""
		switch filter.(type) {
		case *ruled.RuleFilter:
			typ = "ruled"
		case *keyed.Query:
			typ = "keyed"
		default:
			writeError(req, res, fmt.Errorf("unknown filter type %T", filter))
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
		htJSONContent(res, req)
		name := chi.URLParam(req, "name")
		if err := filterdb.Remove(name); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htFilterSet(filterdb *filter.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		htJSONContent(res, req)
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
			writeError(req, res, fmt.Errorf("unknown filter type %q", data.Filter.Type))
			return
		}

		if err := json.Unmarshal([]byte(data.Filter.Value), filter); err != nil {
			writeError(req, res, err)
			return
		}
		name := chi.URLParam(req, "name")
		if err := filterdb.Set(name, filter); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htStreamsList(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		htJSONContent(res, req)
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
		htJSONContent(res, req)
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
		htJSONContent(res, req)
		stream := stream.Stream{Filename: req.FormValue("filename")}
		if err := streamdb.RemoveStream(&stream); err != nil {
			writeError(req, res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}
