package main

import (
	"regexp"
)

var streamsStorage *PersistentStorage
var defaultStreams = []StreamTrack{
	{
		Url:   "http://pub7.di.fm/di_trance",
		Art:   "http://api.audioaddict.com/v1/assets/image/befc1043f0a216128f8570d3664856f7.png?size=200x200",
		Album: "DI Trance",
	},
}

func InitStreams() error {
	ss, err := NewPersistentStorage("streams", &[]StreamTrack{})
	if err != nil {
		return err
	}
	streamsStorage = ss

	if len(GetStreams()) == 0 {
		if err := streamsStorage.SetValue(&defaultStreams); err != nil {
			return err
		}
	}

	return nil
}


type StreamTrack struct {
	Url   string `json:"id"`
	Album string `json:"album",omitempty`
	Title string `json:"title,omitempty"`
	Art   string `json:"art"`
}

func (this *StreamTrack) GetUri() string {
	return this.Url
}

func GetStreams() []StreamTrack {
	return *streamsStorage.Value().(*[]StreamTrack)
}

func GetStreamByURL(url string) *StreamTrack {
	for _, stream := range GetStreams() {
		if stream.Url == url {
			return &stream
		}
	}
	return nil
}

func AddStream(stream *StreamTrack) error {
	if GetStreamByURL(stream.Url) != nil {
		return nil
	}

	streams := append(GetStreams(), *stream)
	return streamsStorage.SetValue(&streams)
}

func RemoveStreamByUrl(url string) error {
	streams := GetStreams()
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
	streams = streams[:len(streams)-found]
	return streamsStorage.SetValue(&streams)
}

func IsStreamUri(uri string) (ok bool) {
	ok, _ = regexp.Match("^https?:\\/\\/", []byte(uri))
	return
}
