package main

import (
	"encoding/json"
	"net/http"
	"regexp"

	"./player"
	"github.com/gorilla/mux"
)

func htDataAttach(r *mux.Router, queuer *player.Queuer, streamdb *player.StreamDB) {
	r.Path("/queuer").Methods("GET").HandlerFunc(htQueuerulesGet(queuer))
	r.Path("/queuer").Methods("POST").HandlerFunc(htQueuerulesSet(queuer))
	r.Path("/streams").Methods("GET").HandlerFunc(htStreamsList(streamdb))
	r.Path("/streams").Methods("POST").HandlerFunc(htStreamsAdd(streamdb))
	r.Path("/streams").Methods("DELETE").HandlerFunc(htStreamsRemove(streamdb))
}

func htStreamsList(streamdb *player.StreamDB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(res).Encode(map[string]interface{}{
			"streams": streamdb.Streams(),
		})
		if err != nil {
			panic(err)
		}
	}
}

func htStreamsAdd(streamdb *player.StreamDB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Stream player.StreamTrack `json:"stream"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := streamdb.AddStream(data.Stream); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htStreamsRemove(streamdb *player.StreamDB) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Stream player.StreamTrack `json:"stream"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := streamdb.RemoveStreamByUrl(data.Stream.Url); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htQueuerulesGet(queuer *player.Queuer) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(res).Encode(map[string]interface{}{
			"queuerules": queuer.Rules(),
		})
		if err != nil {
			panic(err)
		}
	}
}

func htQueuerulesSet(queuer *player.Queuer) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Rules []player.SelectionRule `json:"queuerules"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if err := queuer.SetRules(data.Rules); err != nil {
			if err, ok := err.(player.RuleError); ok {
				res.WriteHeader(400)
				json.NewEncoder(res).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"message":   err.Error(),
						"ruleindex": err.Index,
					},
				})
				return
			}
			panic(err)
		}

		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

// "//" Gets converted to "/" in URLs, resulting in streams not being
// recognised. This is hopefully a temporary fix. :(
func fixStreamUri(uri string) string {
	re := regexp.MustCompile("^([a-z]+):\\/([^\\/])")
	return re.ReplaceAllString(uri, "$1://$2")
}
