package stream

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"../player"
	"../util"
)

type Stream struct {
	Url         string `json:"id"`
	StreamTitle string `json:"title,omitempty"`
	ArtUrl      string `json:"art,omitempty"`
}

func (stream Stream) PlayerTrack() player.Track {
	return player.Track{
		Uri:    stream.Url,
		Title:  stream.StreamTitle,
		HasArt: stream.ArtUrl != "",
	}
}

func (stream Stream) Art() (image io.ReadCloser, mime string) {
	if stream.ArtUrl == "" {
		return nil, ""
	}

	re := regexp.MustCompile("data:([a-zA-Z/]+);base64,(.+)$")
	if match := re.FindStringSubmatch(stream.ArtUrl); len(match) > 0 {
		return ioutil.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(match[2]))), match[1]
	}

	// Legacy
	res, err := http.Get(stream.ArtUrl)
	if err != nil {
		return nil, ""
	}
	return res.Body, res.Header.Get("Content-Type")
}

type DB struct {
	util.Emitter
	storage *util.PersistentStorage
}

func NewDB(file string) (db *DB, err error) {
	db = &DB{}
	if db.storage, err = util.NewPersistentStorage(file, &[]Stream{}); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) Streams() []Stream {
	return *db.storage.Value().(*[]Stream)
}

func (db *DB) Tracks() ([]player.Track, error) {
	streams := db.Streams()
	tracks := make([]player.Track, len(streams))
	for i, stream := range streams {
		tracks[i] = stream.PlayerTrack()
	}
	return tracks, nil
}

func (db *DB) TrackInfo(uris ...string) ([]player.Track, error) {
	tracks := make([]player.Track, len(uris))
	for i, uri := range uris {
		if stream := db.StreamByURL(uri); stream != nil {
			tracks[i] = stream.PlayerTrack()
		}
	}
	return tracks, nil
}

func (db *DB) TrackArt(track string) (image io.ReadCloser, mime string) {
	stream := db.StreamByURL(track)
	if stream == nil {
		return nil, ""
	}
	return stream.Art()
}

func (db *DB) StreamByURL(url string) *Stream {
	for _, stream := range db.Streams() {
		if stream.Url == url {
			return &stream
		}
	}
	return nil
}

func (db *DB) SetStreams(streams []Stream) (err error) {
	err = db.storage.SetValue(&streams)
	db.Emit("update")
	return
}

func (db *DB) AddStreams(streams ...Stream) error {
	errs := make(chan error)
	done := make(chan struct{})
	defer close(errs)
	defer close(done)

	client := http.Client{
		Timeout: time.Second * 30,
	}

	var abortedLock sync.Mutex
	var aborted bool

	initial := db.Streams()

	for i, tr := range streams {
		// Remove duplicates.
		for i, s := range initial {
			if s.Url == tr.Url {
				initial = append(initial[:i], initial[i+1:]...)
			}
		}

		go func(stream *Stream) {
			if stream.ArtUrl != "" && !regexp.MustCompile("^data:").MatchString(stream.ArtUrl) {
				res, err := client.Get(stream.ArtUrl)
				if err != nil {
					abortedLock.Lock()
					ab := aborted
					abortedLock.Unlock()
					if !ab {
						errs <- err
					}
					return
				}
				defer res.Body.Close()

				contentType := res.Header.Get("Content-Type")
				if !regexp.MustCompile("^image/").MatchString(contentType) {
					abortedLock.Lock()
					ab := aborted
					abortedLock.Unlock()
					if !ab {
						errs <- fmt.Errorf("Invalid content type for stream image %s", contentType)
					}
					return
				}
				var buf bytes.Buffer
				fmt.Fprintf(&buf, "data:%s;base64,", contentType)
				if _, err := io.Copy(base64.NewEncoder(base64.StdEncoding, &buf), res.Body); err != nil {
					abortedLock.Lock()
					ab := aborted
					abortedLock.Unlock()
					if !ab {
						errs <- err
					}
					return
				}
				stream.ArtUrl = buf.String()
			}
			abortedLock.Lock()
			ab := aborted
			abortedLock.Unlock()
			if !ab {
				done <- struct{}{}
			}
		}(&streams[i])
	}

	for remaining := len(streams); remaining > 0; {
		select {
		case err := <-errs:
			abortedLock.Lock()
			aborted = true
			abortedLock.Unlock()
			return err
		case <-done:
			remaining--
		}
	}

	return db.SetStreams(append(initial, streams...))
}

func (db *DB) RemoveStreamByUrl(url string) error {
	streams := db.Streams()
	for i, stream := range streams {
		if stream.Url == url {
			return db.SetStreams(append(streams[:i], streams[i+1:]...))
		}
	}
	return nil
}
