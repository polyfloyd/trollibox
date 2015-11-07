package player

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

	"./event"
)

type StreamTrack struct {
	Url         string `json:"id"`
	StreamTitle string `json:"album,omitempty"`
	ArtUrl      string `json:"art,omitempty"`
}

func (track StreamTrack) Uri() string       { return track.Url }
func (StreamTrack) Artist() string          { return "" }
func (track StreamTrack) Title() string     { return track.StreamTitle }
func (StreamTrack) Genre() string           { return "" }
func (StreamTrack) Album() string           { return "" }
func (StreamTrack) AlbumArtist() string     { return "" }
func (StreamTrack) AlbumTrack() string      { return "" }
func (StreamTrack) AlbumDisc() string       { return "" }
func (StreamTrack) Duration() time.Duration { return 0 }

func (track StreamTrack) Art() (image io.ReadCloser, mime string) {
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

type StreamDB struct {
	*event.Emitter
	storage *PersistentStorage
}

func NewStreamDB(file string) (db *StreamDB, err error) {
	db = &StreamDB{
		Emitter: event.NewEmitter(),
	}
	if db.storage, err = NewPersistentStorage(file, &[]StreamTrack{}); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *StreamDB) Streams() []StreamTrack {
	return *db.storage.Value().(*[]StreamTrack)
}

func (db *StreamDB) StreamByURL(url string) *StreamTrack {
	for _, stream := range db.Streams() {
		if stream.Url == url {
			return &stream
		}
	}
	return nil
}

func (db *StreamDB) SetStreams(streams []StreamTrack) error {
	db.Emit("update")
	return db.storage.SetValue(&streams)
}

func (db *StreamDB) AddStream(track StreamTrack) error {
	if db.StreamByURL(track.Url) != nil {
		return nil
	}

	if track.ArtUrl != "" && !regexp.MustCompile("^data:").MatchString(track.ArtUrl) {
		res, err := http.Get(track.ArtUrl)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		contentType := res.Header.Get("Content-Type")
		if !regexp.MustCompile("^image/").MatchString(contentType) {
			return fmt.Errorf("Invalid content type for stream image %s", contentType)
		}
		var buf bytes.Buffer
		if _, err := io.Copy(base64.NewEncoder(base64.StdEncoding, &buf), res.Body); err != nil {
			return err
		}
		track.ArtUrl = fmt.Sprintf("data:%s;base64,%s", contentType, buf.String())
	}

	return db.SetStreams(append(db.Streams(), track))
}

func (db *StreamDB) RemoveStreamByUrl(url string) error {
	streams := db.Streams()
	for i, stream := range streams {
		if stream.Url == url {
			return db.SetStreams(append(streams[:i], streams[i+1:]...))
		}
	}
	return nil
}
