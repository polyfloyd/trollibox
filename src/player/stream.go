package player

import (
	"io"
	"net/http"
	"regexp"

	"./event"
)

type StreamTrack struct {
	Url         string `json:"id"`
	StreamTitle string `json:"album,omitempty"`
	ArtUrl      string `json:"art,omitempty"`
}

func (track StreamTrack) Uri() string   { return track.Url }
func (StreamTrack) Artist() string      { return "" }
func (track StreamTrack) Title() string { return track.StreamTitle }
func (StreamTrack) Genre() string       { return "" }
func (StreamTrack) Album() string       { return "" }
func (StreamTrack) AlbumArtist() string { return "" }
func (StreamTrack) AlbumTrack() string  { return "" }
func (StreamTrack) AlbumDisc() string   { return "" }
func (StreamTrack) Duration() int       { return 0 }

func (track StreamTrack) Art() (image io.ReadCloser, mime string) {
	if track.ArtUrl == "" {
		return nil, ""
	}

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

func (db *StreamDB) AddStream(stream StreamTrack) error {
	if db.StreamByURL(stream.Url) != nil {
		return nil
	}
	return db.SetStreams(append(db.Streams(), stream))
}

func (db *StreamDB) RemoveStreamByUrl(url string) error {
	streams := db.Streams()
	found := 0
	for i, stream := range streams {
		if stream.Url == url {
			found++
		}
		if i+found == len(streams) {
			break
		}
		streams[i] = streams[i+found]
	}
	return db.SetStreams(streams[:len(streams)-found])
}

func IsStreamUri(uri string) (ok bool) {
	ok, _ = regexp.Match("^https?:\\/\\/", []byte(uri))
	return
}
