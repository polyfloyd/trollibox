package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"time"
	ws "golang.org/x/net/websocket"
)

func socketHandler(player *Player) func(*ws.Conn) {
	return func(conn *ws.Conn) {
		ch := make(chan string, 16)
		listenHandle := player.Listen(ch)
		defer close(ch)
		defer player.Unlisten(listenHandle)

		conn.SetDeadline(time.Now().Add(time.Hour * 42 * 42 * 42))

		for {
			_, err := conn.Write([]uint8(<-ch))
			if err != nil {
				break
			}
		}
	}
}

func htDataAttach(r *mux.Router, player *Player) {
	r.Path("/player/state").Methods("POST").HandlerFunc(htPlayerSetState(player))
	r.Path("/player/next").Methods("POST").HandlerFunc(htPlayerNext(player))
	r.Path("/player/progress").Methods("POST").HandlerFunc(htPlayerProgress(player))
	r.Path("/player/volume").Methods("GET").HandlerFunc(htPlayerGetVolume(player))
	r.Path("/player/volume").Methods("POST").HandlerFunc(htPlayerSetVolume(player))
	r.Path("/player/playlist").Methods("GET").HandlerFunc(htPlayerGetPlaylist(player))
	r.Path("/player/playlist").Methods("POST").HandlerFunc(htPlayerSetPlaylist(player))
	r.Path("/player/current").Methods("GET").HandlerFunc(htPlayerCurrentTrack(player))
	r.Path("/track/browse{path:.*}").Methods("GET").HandlerFunc(htPlayerTracks(player))
	r.Path("/track/art/{path:.*}").Methods("GET").HandlerFunc(htTrackArt(player))
	r.Path("/track/art/{path:.*}").Methods("HEAD").HandlerFunc(htTrackArtProbe(player))
	r.Path("/listen").Handler(ws.Handler(socketHandler(player)))
}

func htPlayerNext(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		if err := player.Next(); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htPlayerProgress(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Progress int `json:"progress"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := player.SetProgress(data.Progress); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htPlayerSetState(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			State string `json:"state"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := player.SetState(data.State); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htPlayerGetVolume(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		volume, err := player.Volume()
		if err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"volume": volume,
		})
		if err != nil {
			panic(err)
		}
	}
}

func htPlayerSetVolume(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Volume float32 `json:"volume"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := player.SetVolume(data.Volume); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htPlayerCurrentTrack(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		if track, progress, err := player.CurrentTrack(); err != nil {
			panic(err)
		} else if state, err := player.State(); err != nil {
			panic(err)
		} else {
			res.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(res).Encode(map[string]interface{}{
				"progress": progress,
				"track":    track,
				"state":    state,
			})
			if err != nil {
				panic(err)
			}
		}
	}
}

func htPlayerGetPlaylist(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		tracks, err := player.Playlist()
		if err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": tracks,
		})
		if err != nil {
			panic(err)
		}
	}
}

func htPlayerSetPlaylist(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			TrackIds []string `json:"track-ids"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := player.SetPlaylistIds(data.TrackIds); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htPlayerTracks(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		tracks, err := player.ListTracks(mux.Vars(req)["path"], true)
		if err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(res).Encode(map[string]interface{}{
			"tracks": tracks,
		})
		if err != nil {
			panic(err)
		}
	}
}

func htTrackArt(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		tracks, err := player.ListTracks(mux.Vars(req)["path"], false)
		if err != nil {
			panic(err)
		}

		if len(tracks) == 1 {
			if artStream := tracks[0].GetArt(); artStream != nil {
				res.Header().Set("Content-Type", "image/jpg")
				io.Copy(res, artStream)
				return
			}
		}

		http.NotFound(res, req)
	}
}

func htTrackArtProbe(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		tracks, err := player.ListTracks(mux.Vars(req)["path"], false)
		if err != nil {
			panic(err)
		}

		if len(tracks) != 1 || !tracks[0].HasArt() {
			http.NotFound(res, req)
		}
	}
}
