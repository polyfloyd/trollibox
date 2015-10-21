package main

import (
	"regexp"
)

type StreamTrack struct {
	Url   string `json:"id"`
	Album string `json:"album",omitempty`
	Title string `json:"title,omitempty"`
	Art   string `json:"art"`
}

func (stream *StreamTrack) GetUri() string {
	return stream.Url
}

type StreamDB struct {
	*EventEmitter
	storage *PersistentStorage
}

func NewStreamDB(file string) (db *StreamDB, err error) {
	db = &StreamDB{
		EventEmitter: NewEventEmitter(),
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

func (db *StreamDB) AddStream(stream *StreamTrack) error {
	if db.StreamByURL(stream.Url) != nil {
		return nil
	}
	return db.SetStreams(append(db.Streams(), *stream))
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
