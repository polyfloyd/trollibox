package api

import (
	"encoding/json"
	"net/http"

	"trollibox/src/library"
	"trollibox/src/library/stream"
	"trollibox/src/util/eventsource"

	log "github.com/sirupsen/logrus"
)

func jsonStream(stream stream.Stream) interface{} {
	stream.ArtURI = ""
	return stream // A stream can be converted to JSON as-is.
}

func jsonStreams(streams []stream.Stream) interface{} {
	jj := make([]interface{}, len(streams))
	for i, stream := range streams {
		jj[i] = jsonStream(stream)
	}
	return jj
}

func (api *API) streamsList(w http.ResponseWriter, r *http.Request) {
	streams, err := api.jukebox.StreamDB().Streams()
	if api.mapError(w, r, err) {
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"streams": jsonStreams(streams),
	})
}

func (api *API) streamsAdd(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Stream stream.Stream `json:"stream"`
	}
	if receiveJSONForm(w, r, &data) {
		return
	}

	if data.Stream.ArtURI == "" && data.Stream.Filename != "" {
		// Retain the artwork if no new uri is provided.
		tmpl, err := api.jukebox.StreamDB().StreamByFilename(data.Stream.Filename)
		if api.mapError(w, r, err) {
			return
		}
		data.Stream.ArtURI = tmpl.ArtURI
	}
	if err := api.jukebox.StreamDB().StoreStream(&data.Stream); api.mapError(w, r, err) {
		return
	}

	_, _ = w.Write([]byte("{}"))
}

func (api *API) streamsRemove(w http.ResponseWriter, r *http.Request) {
	stream := stream.Stream{Filename: r.FormValue("filename")}
	if err := api.jukebox.StreamDB().RemoveStream(&stream); api.mapError(w, r, err) {
		return
	}

	_, _ = w.Write([]byte("{}"))
}

func (api *API) streamEvents(w http.ResponseWriter, r *http.Request) {
	es, err := eventsource.Begin(w, r)
	if api.mapError(w, r, err) {
		return
	}
	listener := api.jukebox.StreamDB().Listen()
	defer api.jukebox.StreamDB().Unlisten(listener)

	streams, err := api.jukebox.StreamDB().Streams()
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	es.EventJSON("streams", map[string]interface{}{"streams": jsonStreams(streams)})

	for {
		var event interface{}
		select {
		case event = <-listener:
		case <-r.Context().Done():
			return
		}

		switch event.(type) {
		case library.UpdateEvent:
			streams, err := api.jukebox.StreamDB().Streams()
			if err != nil {
				log.Errorf("%v", err)
				return
			}
			es.EventJSON("streams", map[string]interface{}{"streams": jsonStreams(streams)})

		default:
			log.Debugf("Unmapped stream db event %#v", event)
		}
	}
}
