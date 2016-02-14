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
	"time"

	"../player"
	"../util"
)

type Stream struct {
	Url         string `json:"url"`
	StreamTitle string `json:"title,omitempty"`
	ArtUrl      string `json:"art,omitempty"`
}

func (stream *Stream) PlayerTrack() player.Track {
	return player.Track{
		Uri:    stream.Url,
		Title:  stream.StreamTitle,
		HasArt: stream.ArtUrl != "",
	}
}

func (stream *Stream) Art() (image io.ReadCloser, mime string) {
	re := regexp.MustCompile("data:([a-zA-Z/]+);base64,(.+)$")
	if match := re.FindStringSubmatch(stream.ArtUrl); len(match) > 0 {
		return ioutil.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(match[2]))), match[1]
	}
	return nil, ""
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
		if stream := db.streamByURL(uri); stream != nil {
			tracks[i] = stream.PlayerTrack()
		}
	}
	return tracks, nil
}

func (db *DB) TrackArt(track string) (image io.ReadCloser, mime string) {
	stream := db.streamByURL(track)
	if stream == nil {
		return nil, ""
	}
	return stream.Art()
}

func (db *DB) AddStream(stream Stream) error {
	if db.streamByURL(stream.Url) != nil {
		return nil
	}

	// Download the track art and store it as a data URI.
	if stream.ArtUrl != "" && !regexp.MustCompile("^data:").MatchString(stream.ArtUrl) {
		client := http.Client{Timeout: time.Second * 30}
		res, err := client.Get(stream.ArtUrl)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		contentType := res.Header.Get("Content-Type")
		if !regexp.MustCompile("^image/").MatchString(contentType) {
			return fmt.Errorf("Invalid content type for stream image %s", contentType)
		}
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "data:%s;base64,", contentType)
		if _, err := io.Copy(base64.NewEncoder(base64.StdEncoding, &buf), res.Body); err != nil {
			return err
		}
		stream.ArtUrl = buf.String()
	}

	return db.setStreams(append(db.Streams(), stream))
}

func (db *DB) RemoveStreamByUrl(url string) error {
	streams := db.Streams()
	for i, stream := range streams {
		if stream.Url == url {
			return db.setStreams(append(streams[:i], streams[i+1:]...))
		}
	}
	return nil
}

func (db *DB) streamByURL(url string) *Stream {
	for _, stream := range db.Streams() {
		if stream.Url == url {
			return &stream
		}
	}
	return nil
}

func (db *DB) setStreams(streams []Stream) (err error) {
	err = db.storage.SetValue(&streams)
	db.Emit("update")
	return
}
