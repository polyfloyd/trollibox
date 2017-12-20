package raw

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"sync"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/util"
)

type rawTrack struct {
	server *Server
	id     uint64
	name   string
	buffer *util.BlockingBuffer

	image     []byte
	imageMime string
}

func (rt *rawTrack) track() library.Track {
	return library.Track{
		URI:    fmt.Sprintf("%s?track=%d", rt.server.urlRoot, rt.id),
		Title:  rt.name,
		HasArt: rt.image != nil,
	}
}

// A Server stores audio files and acts as a library for these files, exposing
// their contents over HTTP.
type Server struct {
	util.Emitter
	urlRoot    string
	idEnum     uint64
	tracks     map[uint64]rawTrack
	tracksLock sync.RWMutex
}

// NewServer creates a new server that configures tracks using the specified
// URL-root.
func NewServer(urlRoot string) *Server {
	return &Server{
		urlRoot: urlRoot,
		tracks:  map[uint64]rawTrack{},
	}
}

func (sv *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	sv.tracksLock.RLock()
	id, _ := strconv.ParseUint(req.FormValue("track"), 10, 64)
	track, ok := sv.tracks[id]
	sv.tracksLock.RUnlock()

	if !ok {
		http.NotFound(res, req)
		return
	}
	res.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(track.name)))
	r := track.buffer.Reader()
	defer r.Close()
	io.Copy(res, r)
}

// Add reads an audio file and creates a track with a title and optional image.
func (sv *Server) Add(inputFile io.ReadCloser, title string, image []byte, imageMime string) (library.Track, <-chan error) {
	bbuf, err := util.NewBlockingBuffer()
	if err != nil {
		inputFile.Close()
		return library.Track{}, util.ErrorAsChannel(fmt.Errorf("error adding raw track: %v", err))
	}
	track := rawTrack{
		server:    sv,
		name:      title,
		buffer:    bbuf,
		image:     image,
		imageMime: imageMime,
	}
	sv.tracksLock.Lock()
	sv.idEnum++
	track.id = sv.idEnum
	sv.tracks[track.id] = track
	sv.tracksLock.Unlock()
	sv.Emit(library.UpdateEvent{})

	errc := make(chan error, 1)
	go func() {
		defer inputFile.Close()
		defer close(errc)
		if _, err := io.Copy(track.buffer, inputFile); err != nil {
			track.buffer.Destroy()
			sv.tracksLock.Lock()
			delete(sv.tracks, track.id)
			sv.tracksLock.Unlock()
			sv.Emit(library.UpdateEvent{})
			errc <- fmt.Errorf("error adding raw track: %v", err)
			return
		}
	}()
	return track.track(), errc
}

// Remove removes a track managed by server.
//
// This is a no-op if no track with the given URL is found.
func (sv *Server) Remove(uri string) error {
	sv.tracksLock.Lock()
	defer sv.tracksLock.Unlock()

	trackID := idFromURL(uri)
	rt, ok := sv.tracks[trackID]
	if !ok {
		return nil
	}
	rt.buffer.Destroy()
	delete(sv.tracks, trackID)

	sv.Emit(library.UpdateEvent{})
	return nil
}

// Tracks implements the library.Library interface.
func (sv *Server) Tracks() ([]library.Track, error) {
	sv.tracksLock.RLock()
	defer sv.tracksLock.RUnlock()

	tracks := make([]library.Track, 0, len(sv.tracks))
	for _, rt := range sv.tracks {
		tracks = append(tracks, rt.track())
	}
	return tracks, nil
}

// TrackInfo implements the library.Library interface.
func (sv *Server) TrackInfo(uris ...string) ([]library.Track, error) {
	sv.tracksLock.RLock()
	defer sv.tracksLock.RUnlock()

	tracks := make([]library.Track, len(uris))
	for i, uri := range uris {
		if rt, ok := sv.tracks[idFromURL(uri)]; ok {
			tracks[i] = rt.track()
		}
	}
	return tracks, nil
}

// TrackArt implements the library.Library interface.
func (sv *Server) TrackArt(uri string) (io.ReadCloser, string) {
	sv.tracksLock.RLock()
	defer sv.tracksLock.RUnlock()

	track := sv.tracks[idFromURL(uri)]
	if track.image == nil {
		return nil, ""
	}
	return ioutil.NopCloser(bytes.NewReader(track.image)), track.imageMime
}

// Events implements the player.Player interface.
func (sv *Server) Events() *util.Emitter {
	return &sv.Emitter
}

func idFromURL(url string) uint64 {
	m := regexp.MustCompile("\\?track=(\\d+)$").FindStringSubmatch(url)
	if m == nil {
		return 0
	}
	id, _ := strconv.ParseUint(m[1], 10, 64)
	return id
}
