package api

import (
	"encoding/json"
	"net/http"

	"github.com/polyfloyd/trollibox/src/library/stream"
)

type streamsAPI struct {
	db *stream.DB
}

func (api *streamsAPI) list(res http.ResponseWriter, req *http.Request) {
	streams, err := api.db.Streams()
	if err != nil {
		WriteError(req, res, err)
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

func (api *streamsAPI) add(res http.ResponseWriter, req *http.Request) {
	var data struct {
		Stream stream.Stream `json:"stream"`
	}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		WriteError(req, res, err)
		return
	}

	if data.Stream.ArtURI == "" && data.Stream.Filename != "" {
		// Retain the artwork if no new uri is provided.
		tmpl, err := api.db.StreamByFilename(data.Stream.Filename)
		if err != nil {
			WriteError(req, res, err)
			return
		}
		data.Stream.ArtURI = tmpl.ArtURI
	}

	if err := api.db.StoreStream(&data.Stream); err != nil {
		WriteError(req, res, err)
		return
	}
	res.Write([]byte("{}"))
}

func (api *streamsAPI) remove(res http.ResponseWriter, req *http.Request) {
	stream := stream.Stream{Filename: req.FormValue("filename")}
	if err := api.db.RemoveStream(&stream); err != nil {
		WriteError(req, res, err)
		return
	}
	res.Write([]byte("{}"))
}
