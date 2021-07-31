package api

import (
	"encoding/json"
	"net/http"

	"trollibox/src/library/stream"
)

func (api *API) streamsList(w http.ResponseWriter, r *http.Request) {
	streams, err := api.jukebox.StreamDB().Streams()
	if err != nil {
		WriteError(w, r, err)
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"streams": mapped,
	})
}

func (api *API) streamsAdd(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Stream stream.Stream `json:"stream"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		WriteError(w, r, err)
		return
	}

	if data.Stream.ArtURI == "" && data.Stream.Filename != "" {
		// Retain the artwork if no new uri is provided.
		tmpl, err := api.jukebox.StreamDB().StreamByFilename(data.Stream.Filename)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		data.Stream.ArtURI = tmpl.ArtURI
	}

	if err := api.jukebox.StreamDB().StoreStream(&data.Stream); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}

func (api *API) streamsRemove(w http.ResponseWriter, r *http.Request) {
	stream := stream.Stream{Filename: r.FormValue("filename")}
	if err := api.jukebox.StreamDB().RemoveStream(&stream); err != nil {
		WriteError(w, r, err)
		return
	}
	w.Write([]byte("{}"))
}
