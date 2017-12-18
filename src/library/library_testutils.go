package library

import (
	"io"
)

// DummyLibrary is used for testing.
type DummyLibrary []Track

// Tracks implements the library.Library interface.
func (lib *DummyLibrary) Tracks() ([]Track, error) {
	return *lib, nil
}

// TrackInfo implements the library.Library interface.
func (lib *DummyLibrary) TrackInfo(uris ...string) ([]Track, error) {
	tracks := make([]Track, len(uris))
	for i, uri := range uris {
		for _, track := range *lib {
			if uri == track.URI {
				tracks[i] = track
			}
		}
	}
	return tracks, nil
}

// TrackArt implements the library.Library interface.
func (lib *DummyLibrary) TrackArt(uri string) (image io.ReadCloser, mime string) {
	return nil, ""
}
