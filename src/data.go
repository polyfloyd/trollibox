package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

func htDataAttach(r *mux.Router, player *Player) {
	r.Path("/track/current").Methods("GET").HandlerFunc(htPlayerCurrentTrack(player))
	r.Path("/track/browse{path:.*}").Methods("GET").HandlerFunc(htPlayerTracks(player))
}

func htPlayerCurrentTrack(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		if track, progress, err := player.CurrentTrack(); err != nil {
			panic(err)
		} else {
			err := json.NewEncoder(res).Encode(map[string]interface{}{
				"progress": progress,
				"track":    track,
			})
			if err != nil {
				panic(err)
			}
		}
	}
}

func htPlayerTracks(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		tracks, err := player.ListTracks(mux.Vars(req)["path"])
		if err != nil {
			panic(err)
		}

		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": tracks,
		})
		if err != nil {
			panic(err)
		}
	}
}
