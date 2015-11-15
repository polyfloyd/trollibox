package main

import (
	"encoding/json"
	"net/http"

	"./player"
	"./stream"
	"./stream/digitallyimported"
	"github.com/gorilla/mux"
)

func htDataAttach(r *mux.Router, queuer *player.Queuer, streamdb *stream.DB) {
	r.Path("/queuer").Methods("GET").HandlerFunc(htQueuerulesGet(queuer))
	r.Path("/queuer").Methods("POST").HandlerFunc(htQueuerulesSet(queuer))
	r.Path("/streams").Methods("GET").HandlerFunc(htStreamsList(streamdb))
	r.Path("/streams").Methods("POST").HandlerFunc(htStreamsAdd(streamdb))
	r.Path("/streams").Methods("DELETE").HandlerFunc(htStreamsRemove(streamdb))
	r.Path("/streams/loaddefault").Methods("POST").HandlerFunc(htStreamsLoadDefaults(streamdb))
}

// Writes an error to the client or an empty object if err is nil.
func writeError(res http.ResponseWriter, err error) {
	if err == nil {
		res.Write([]byte("{}"))
	}
	json.NewEncoder(res).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func htStreamsList(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		json.NewEncoder(res).Encode(map[string]interface{}{
			"streams": streamdb.Streams(),
		})
	}
}

func htStreamsAdd(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Stream stream.Track `json:"stream"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := streamdb.AddStreams(data.Stream); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htStreamsLoadDefaults(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		streams, err := digitallyimported.Streams()
		if err != nil {
			writeError(res, err)
			return
		}

		if err := streamdb.AddStreams(streams...); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htStreamsRemove(streamdb *stream.DB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Stream stream.Track `json:"stream"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := streamdb.RemoveStreamByUrl(data.Stream.Url); err != nil {
			writeError(res, err)
			return
		}
		res.Write([]byte("{}"))
	}
}

func htQueuerulesGet(queuer *player.Queuer) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		json.NewEncoder(res).Encode(map[string]interface{}{
			"queuerules": queuer.Rules(),
		})
	}
}

func htQueuerulesSet(queuer *player.Queuer) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		var data struct {
			Rules []player.SelectionRule `json:"queuerules"`
		}
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			writeError(res, err)
			return
		}

		if err := queuer.SetRules(data.Rules); err != nil {
			if err, ok := err.(*player.RuleError); ok {
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
