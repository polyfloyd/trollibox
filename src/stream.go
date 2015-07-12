package main

import (
	"regexp"
)

var streams = []StreamTrack{
	{
		Url:   "http://pub7.di.fm/di_trance",
		Art:   "http://api.audioaddict.com/v1/assets/image/befc1043f0a216128f8570d3664856f7.png?size=200x200",
		Album: "DI Trance",
	},
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
	return streams
}

func GetStreamByURL(url string) *StreamTrack {
	for _, stream := range GetStreams() {
		if stream.Url == url {
			return &stream
		}
	}
	return nil
}

func isStreamUri(uri string) (ok bool) {
	ok, _ = regexp.Match("^https?:\\/\\/", []byte(uri))
	return
}
