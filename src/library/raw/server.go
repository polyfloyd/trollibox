package player

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"../../player"
)

type rawTrack struct {
	file *os.File
	name string

	image     bytes.Buffer
	imageMime string
}

type Server struct {
	urlRoot string
	tmpDir  string
	tracks  map[string]rawTrack
	lock    sync.RWMutex
}

func NewServer(urlRoot string) (*Server, error) {
	tmpDir := path.Join(os.TempDir(), "trollibox-raw")
	if err := os.MkdirAll(tmpDir, 0755|os.ModeTemporary); err != nil {
		return nil, fmt.Errorf("Error creating raw server: %v", err)
	}
	return &Server{
		urlRoot: urlRoot,
		tmpDir:  tmpDir,
		tracks:  map[string]rawTrack{},
	}, nil
}

func (rp *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	trackId := req.FormValue("track")

	rp.lock.RLock()
	track, ok := rp.tracks[trackId]
	rp.lock.RUnlock()

	if !ok {
		http.NotFound(res, req)
		return
	}
	res.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(track.name)))
	http.ServeContent(res, req, trackId, time.Now(), track.file)
}

func (rp *Server) Add(r io.Reader, title string, image io.Reader, imageMime string) (player.Track, error) {
	file, err := ioutil.TempFile(rp.tmpDir, "")
	if err != nil {
		return player.Track{}, fmt.Errorf("Error adding raw track: %v", err)
	}
	trackId := path.Base(file.Name())
	if _, err := io.Copy(file, r); err != nil {
		file.Close()
		os.Remove(file.Name())
		return player.Track{}, fmt.Errorf("Error adding raw track: %v", err)
	}

	track := rawTrack{
		file:      file,
		name:      title,
		imageMime: imageMime,
	}
	if image != nil && imageMime != "" {
		if _, err := io.Copy(&track.image, image); err != nil {
			return player.Track{}, err
		}
	}

	rp.lock.Lock()
	rp.tracks[trackId] = track
	rp.lock.Unlock()
	return player.Track{Uri: fmt.Sprintf("%s?track=%s", rp.urlRoot, trackId)}, nil
}

func (rp *Server) Tracks() ([]player.Track, error) {
	rp.lock.RLock()
	defer rp.lock.RUnlock()

	tracks := make([]player.Track, 0, len(rp.tracks))
	for trackId, rt := range rp.tracks {
		tracks = append(tracks, player.Track{
			Uri:   fmt.Sprintf("%s?track=%s", rp.urlRoot, trackId),
			Title: rt.name,
		})
	}
	return tracks, nil
}

func (rp *Server) TrackInfo(uris ...string) ([]player.Track, error) {
	rp.lock.RLock()
	defer rp.lock.RUnlock()

	tracks := make([]player.Track, len(uris))
	for i, uri := range uris {
		trackId := rawIdFromUrl(uri)
		if tr, ok := rp.tracks[trackId]; ok {
			tracks[i] = player.Track{
				Uri:   uri,
				Title: tr.name,
			}
		}
	}
	return tracks, nil
}

func (rp *Server) TrackArt(uri string) (io.ReadCloser, string) {
	rp.lock.RLock()
	defer rp.lock.RUnlock()
	track := rp.tracks[rawIdFromUrl(uri)]
	if track.imageMime == "" {
		return nil, ""
	}
	return ioutil.NopCloser(bytes.NewReader(track.image.Bytes())), track.imageMime
}

func (rp *Server) Remove(track player.Track) error {
	rp.lock.Lock()
	defer rp.lock.Unlock()

	trackId := rawIdFromUrl(track.Uri)
	rt, ok := rp.tracks[trackId]
	if !ok {
		return nil
	}
	rt.file.Close()
	if err := os.Remove(rt.file.Name()); err != nil {
		return err
	}
	delete(rp.tracks, trackId)
	return nil
}

func rawIdFromUrl(url string) string {
	m := regexp.MustCompile("\\?track=(\\w+)").FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}
