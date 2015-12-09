package player

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"sync"
	"time"
)

type ReadSeekCloser interface {
	io.Closer
	io.ReadSeeker
}

type RawTrackServer struct {
	UrlRoot string
	TmpDir  string

	tracks map[string]*os.File
	lock   sync.Mutex
}

func (rp *RawTrackServer) init() {
	if rp.tracks == nil {
		rp.lock.Lock()
		if rp.tracks == nil {
			rp.tracks = map[string]*os.File{}
		}
		if rp.TmpDir == "" {
			rp.TmpDir = path.Join(os.TempDir(), "trollibox-raw")
		}
		os.MkdirAll(rp.TmpDir, 0755|os.ModeTemporary)
		rp.lock.Unlock()
	}
}

func (rp *RawTrackServer) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	trackId := req.FormValue("track")
	rp.lock.Lock()
	track, ok := rp.tracks[trackId]
	rp.lock.Unlock()
	if !ok {
		http.NotFound(res, req)
		return
	}
	http.ServeContent(res, req, trackId, time.Now(), track)
}

func (rp *RawTrackServer) Add(r io.Reader) (Track, error) {
	rp.init()

	file, err := ioutil.TempFile(rp.TmpDir, "")
	if err != nil {
		return Track{}, err
	}
	trackId := path.Base(file.Name())
	io.Copy(file, r)

	rp.lock.Lock()
	rp.tracks[trackId] = file
	rp.lock.Unlock()

	return Track{Uri: fmt.Sprintf("%s?track=%s", rp.UrlRoot, trackId)}, nil
}

func (rp *RawTrackServer) Tracks() []Track {
	rp.init()
	rp.lock.Lock()
	defer rp.lock.Unlock()

	tracks := make([]Track, 0, len(rp.tracks))
	for trackId := range rp.tracks {
		tracks = append(tracks, Track{Uri: fmt.Sprintf("%s?track=%s", rp.UrlRoot, trackId)})
	}
	return tracks
}

func (rp *RawTrackServer) Remove(track Track) error {
	rp.init()
	rp.lock.Lock()
	defer rp.lock.Unlock()

	m := regexp.MustCompile("\\?track=(\\d+)").FindStringSubmatch(track.Uri)
	if m == nil {
		return nil
	}
	trackId := m[1]

	file, ok := rp.tracks[trackId]
	if !ok {
		return nil
	}

	name := file.Name()
	file.Close()
	if err := os.Remove(name); err != nil {
		return err
	}
	delete(rp.tracks, trackId)
	return nil
}
