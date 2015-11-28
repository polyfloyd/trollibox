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

type Track struct {
	Url         string `json:"id"`
	StreamTitle string `json:"album,omitempty"`
	ArtUrl      string `json:"art,omitempty"`
}

func (track Track) PlayerTrack() player.Track {
	return player.Track{
		Uri:    track.Url,
		Album:  track.StreamTitle,
		HasArt: track.ArtUrl != "",
	}
}

func (track Track) Art() (image io.ReadCloser, mime string) {
	if track.ArtUrl == "" {
		return nil, ""
	}

	re := regexp.MustCompile("data:([a-zA-Z/]+);base64,(.+)$")
	if match := re.FindStringSubmatch(track.ArtUrl); len(match) > 0 {
		return ioutil.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(match[2]))), match[1]
	}

	// Legacy
	res, err := http.Get(track.ArtUrl)
	if err != nil {
		return nil, ""
	}
	return res.Body, res.Header.Get("Content-Type")
}

type DB struct {
	*util.Emitter
	storage *util.PersistentStorage
}

func NewDB(file string) (db *DB, err error) {
	db = &DB{
		Emitter: util.NewEmitter(),
	}
	if db.storage, err = util.NewPersistentStorage(file, &[]Track{}); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) Streams() []Track {
	return *db.storage.Value().(*[]Track)
}

func (db *DB) StreamByURL(url string) *Track {
	for _, stream := range db.Streams() {
		if stream.Url == url {
			return &stream
		}
	}
	return nil
}

func (db *DB) SetStreams(streams []Track) (err error) {
	err = db.storage.SetValue(&streams)
	db.Emit("update")
	return
}

func (db *DB) AddStreams(tracks ...Track) error {
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

	for i, tr := range tracks {
		// Remove duplicates.
		for i, s := range initial {
			if s.Url == tr.Url {
				initial = append(initial[:i], initial[i+1:]...)
			}
		}

		go func(track *Track) {
			if track.ArtUrl != "" && !regexp.MustCompile("^data:").MatchString(track.ArtUrl) {
				res, err := client.Get(track.ArtUrl)
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
				track.ArtUrl = buf.String()
			}
			abortedLock.Lock()
			ab := aborted
			abortedLock.Unlock()
			if !ab {
				done <- struct{}{}
			}
		}(&tracks[i])
	}

	for remaining := len(tracks); remaining > 0; {
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

	return db.SetStreams(append(initial, tracks...))
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
