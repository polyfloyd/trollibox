package player

import (
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
)

type rawTrack struct {
	file *os.File
	name string
}

type RawTrackServer struct {
	urlRoot string
	tmpDir  string
	tracks  map[string]rawTrack
	lock    sync.RWMutex
}

func NewRawTrackServer(urlRoot string) (*RawTrackServer, error) {
	tmpDir := path.Join(os.TempDir(), "trollibox-raw")
	if err := os.MkdirAll(tmpDir, 0755|os.ModeTemporary); err != nil {
		return nil, fmt.Errorf("Error creating raw server: %v", err)
	}
	return &RawTrackServer{
		urlRoot: urlRoot,
		tmpDir:  tmpDir,
		tracks:  map[string]rawTrack{},
	}, nil
}

func (rp *RawTrackServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
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

func (rp *RawTrackServer) Add(r io.Reader, title string) (Track, error) {
	file, err := ioutil.TempFile(rp.tmpDir, "")
	if err != nil {
		return Track{}, fmt.Errorf("Error adding raw track: %v", err)
	}
	trackId := path.Base(file.Name())
	if _, err := io.Copy(file, r); err != nil {
		file.Close()
		os.Remove(file.Name())
		return Track{}, fmt.Errorf("Error adding raw track: %v", err)
	}

	rp.lock.Lock()
	rp.tracks[trackId] = rawTrack{
		file: file,
		name: title,
	}
	rp.lock.Unlock()

	return Track{Uri: fmt.Sprintf("%s?track=%s", rp.urlRoot, trackId)}, nil
}

func (rp *RawTrackServer) Tracks() ([]Track, error) {
	rp.lock.RLock()
	defer rp.lock.RUnlock()

	tracks := make([]Track, 0, len(rp.tracks))
	for trackId, rt := range rp.tracks {
		tracks = append(tracks, Track{
			Uri:   fmt.Sprintf("%s?track=%s", rp.urlRoot, trackId),
			Title: rt.name,
		})
	}
	return tracks, nil
}

func (rp *RawTrackServer) TrackInfo(uris ...string) ([]Track, error) {
	rp.lock.RLock()
	defer rp.lock.RUnlock()

	tracks := make([]Track, len(uris))
	for i, uri := range uris {
		trackId := rawIdFromUrl(uri)
		if tr, ok := rp.tracks[trackId]; ok {
			tracks[i] = Track{
				Uri:   uri,
				Title: tr.name,
			}
		}
	}
	return tracks, nil
}

func (rp *RawTrackServer) TrackArt(track string) (io.ReadCloser, string) {
	return nil, ""
}

func (rp *RawTrackServer) Remove(track Track) error {
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
