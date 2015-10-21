package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/net/websocket"
)

// Dirty hack to remove an extra "Track" JSON object when serializing.
func jsonType(plTr *PlaylistTrack) interface{} {
	if plTr == nil {
		return nil
	}

	if tr, ok := plTr.Track.(*StreamTrack); ok {
		return &struct {
			*StreamTrack
			*QueueAttrs
		}{
			StreamTrack: tr,
			QueueAttrs:  &plTr.QueueAttrs,
		}
	} else if tr, ok := plTr.Track.(*LocalTrack); ok {
		return &struct {
			*LocalTrack
			*QueueAttrs
		}{
			LocalTrack: tr,
			QueueAttrs: &plTr.QueueAttrs,
		}
	} else {
		panic("Unknown track type")
	}
	return nil
}

func jsonTypeList(inList []PlaylistTrack) (outList []interface{}) {
	outList = make([]interface{}, len(inList))
	for i, plTr := range inList {
		outList[i] = jsonType(&plTr)
	}
	return
}

func socketHandler(player *Player) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		ch := player.Listen()
		defer player.Unlisten(ch)

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
	r.Path("/queuer").Methods("GET").HandlerFunc(htQueuerulesGet(player))
	r.Path("/queuer").Methods("POST").HandlerFunc(htQueuerulesSet(player))
	r.Path("/streams").Methods("GET").HandlerFunc(htStreamsList(player))
	r.Path("/streams").Methods("POST").HandlerFunc(htStreamsAdd(player))
	r.Path("/streams").Methods("DELETE").HandlerFunc(htStreamsRemove(player))
	r.Path("/listen").Handler(websocket.Handler(socketHandler(player)))
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
				"track":    jsonType(track),
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
			"tracks": jsonTypeList(tracks),
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
	getResponse := func(path string, wr io.Writer) error {
		tracks, err := player.ListTracks(path, true)
		if err != nil {
			return err
		}

		return json.NewEncoder(wr).Encode(map[string]interface{}{
			"tracks": tracks,
		})
	}

	// Only cache the root since it is the most commonly requested path.
	var cachedRoot *bytes.Buffer
	var cacheMutex sync.Mutex
	go func() {
		listener := player.Listen()
		defer player.Unlisten(listener)
		listener <- "update" // Bootstrap the cycle

		for {
			if event := <-listener; event != "update" {
				continue
			}

			cacheMutex.Lock()
			var newCachedRoot bytes.Buffer
			getResponse("/", &newCachedRoot)
			cachedRoot = &newCachedRoot
			cacheMutex.Unlock()
		}
	}()

	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		if path := mux.Vars(req)["path"]; path == "" || path == "/" {
			cacheMutex.Lock()
			buf := cachedRoot
			cacheMutex.Unlock()
			if _, err := res.Write(buf.Bytes()); err != nil {
				panic(err)
			}
		} else {
			if err := getResponse(path, res); err != nil {
				panic(err)
			}
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

func htStreamsList(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(res).Encode(map[string]interface{}{
			"streams": player.StreamDB().Streams(),
		})
		if err != nil {
			panic(err)
		}
	}
}

func htStreamsAdd(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Stream StreamTrack `json:"stream"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := player.StreamDB().AddStream(&data.Stream); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htStreamsRemove(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Stream StreamTrack `json:"stream"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		if err := player.StreamDB().RemoveStreamByUrl(data.Stream.Url); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if _, err := res.Write([]byte("{}")); err != nil {
			panic(err)
		}
	}
}

func htQueuerulesGet(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(res).Encode(map[string]interface{}{
			"queuerules": player.Queuer().Rules(),
		})
		if err != nil {
			panic(err)
		}
	}
}

func htQueuerulesSet(player *Player) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Rules []SelectionRule `json:"queuerules"`
		}

		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			panic(err)
		}

		res.Header().Set("Content-Type", "application/json")
		if err := player.Queuer().SetRules(data.Rules); err != nil {
			if err, ok := err.(RuleError); ok {
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
