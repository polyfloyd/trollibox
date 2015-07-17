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

func (this *StreamTrack) GetUri() string {
	return this.Url
}


type StreamDB struct {
	*EventEmitter
	storage *PersistentStorage
}

func NewStreamDB(file string) (this *StreamDB, err error) {
	this = &StreamDB{
		EventEmitter: NewEventEmitter(),
	}
	if this.storage, err = NewPersistentStorage(file, &[]StreamTrack{}); err != nil {
		return nil, err
	}

	return this, nil
}

func (this *StreamDB) Streams() []StreamTrack {
	return *this.storage.Value().(*[]StreamTrack)
}

func (this *StreamDB) StreamByURL(url string) *StreamTrack {
	for _, stream := range this.Streams() {
		if stream.Url == url {
			return &stream
		}
	}
	return nil
}

func (this *StreamDB) SetStreams(streams []StreamTrack) error {
	this.Emit("update")
	return this.storage.SetValue(&streams)
}

func (this *StreamDB) AddStream(stream *StreamTrack) error {
	if this.StreamByURL(stream.Url) != nil {
		return nil
	}
	return this.SetStreams(append(this.Streams(), *stream))
}

func (this *StreamDB) RemoveStreamByUrl(url string) error {
	streams := this.Streams()
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
	return this.SetStreams(streams[:len(streams)-found])
}

func IsStreamUri(uri string) (ok bool) {
	ok, _ = regexp.Match("^https?:\\/\\/", []byte(uri))
	return
}
