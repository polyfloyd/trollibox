package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"./filter/ruled"
	"./player"
	"./stream"
	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)

func htDataAttach(r *mux.Router, queuerdb *ruled.DB, streamdb *stream.DB, rawServer *player.RawTrackServer) {
	r.Path("/queuer").Methods("GET").HandlerFunc(htQueuerulesGet(queuerdb))
	r.Path("/queuer").Methods("POST").HandlerFunc(htQueuerulesSet(queuerdb))
	r.Path("/streams/listen").Handler(websocket.Handler(htStreamsListen(streamdb)))
	r.Path("/streams").Methods("GET").HandlerFunc(htStreamsList(streamdb))
	r.Path("/streams").Methods("POST").HandlerFunc(htStreamsAdd(streamdb))
	r.Path("/streams").Methods("DELETE").HandlerFunc(htStreamsRemove(streamdb))
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

func htStreamsListen(streamdb *stream.DB) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		strCh := streamdb.Listen()
		defer streamdb.Unlisten(strCh)
		conn.SetDeadline(time.Time{})
		for {
			if _, err := conn.Write([]uint8(<-strCh)); err != nil {
				break
			}
		}
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

func htQueuerulesGet(queuerdb *ruled.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		json.NewEncoder(res).Encode(map[string]interface{}{
			"queuerules": queuerdb.Rules(),
		})
	}
}

func htQueuerulesSet(queuerdb *ruled.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Rules []ruled.Rule `json:"queuerules"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := queuerdb.SetRules(data.Rules); err != nil {
			if err, ok := err.(*ruled.RuleError); ok {
				res.WriteHeader(400)
				json.NewEncoder(res).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"message":   err.Error(),
						"ruleindex": err.Index,
					},
				})
				return
			}
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}
