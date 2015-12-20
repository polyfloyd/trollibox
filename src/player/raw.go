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
	UrlRoot string
	TmpDir  string

	tracks map[string]rawTrack
	lock   sync.Mutex
}

func (rp *RawTrackServer) init() {
	if rp.tracks == nil {
		rp.lock.Lock()
		if rp.tracks == nil {
			rp.tracks = map[string]rawTrack{}
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
	res.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(track.name)))
	http.ServeContent(res, req, trackId, time.Now(), track.file)
}

func (rp *RawTrackServer) Add(r io.Reader, title string) (Track, error) {
	rp.init()

	file, err := ioutil.TempFile(rp.TmpDir, "")
	if err != nil {
		return Track{}, err
	}
	trackId := path.Base(file.Name())
	io.Copy(file, r)

	rp.lock.Lock()
	rp.tracks[trackId] = rawTrack{
		file: file,
		name: title,
	}
	rp.lock.Unlock()

	return Track{Uri: fmt.Sprintf("%s?track=%s", rp.UrlRoot, trackId)}, nil
}

func (rp *RawTrackServer) Tracks() ([]Track, error) {
	rp.init()
	rp.lock.Lock()
	defer rp.lock.Unlock()

	tracks := make([]Track, 0, len(rp.tracks))
	for trackId, rt := range rp.tracks {
		tracks = append(tracks, Track{
			Uri:   fmt.Sprintf("%s?track=%s", rp.UrlRoot, trackId),
			Title: rt.name,
		})
	}
	return tracks, nil
}

func (rp *RawTrackServer) TrackInfo(identites ...TrackIdentity) ([]Track, error) {
	rp.init()
	rp.lock.Lock()
	defer rp.lock.Unlock()

	tracks := make([]Track, len(identites))
	for i, id := range identites {
		trackId := rawIdFromUrl(id.TrackUri())
		if tr, ok := rp.tracks[trackId]; ok {
			tracks[i] = Track{
				Uri:   id.TrackUri(),
				Title: tr.name,
			}
		}
	}
	return tracks, nil
}

func (rp *RawTrackServer) TrackArt(track TrackIdentity) (io.ReadCloser, string) {
	return nil, ""
}

func (rp *RawTrackServer) Remove(track Track) error {
	rp.init()
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
	m := regexp.MustCompile("\\?track=(\\d+)").FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}
