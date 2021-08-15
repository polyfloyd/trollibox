package library

import (
	"context"
	"io"

	"trollibox/src/util"
)

// DummyLibrary is used for testing.
type DummyLibrary []Track

// Tracks implements the library.Library interface.
func (lib *DummyLibrary) Tracks(ctx context.Context) ([]Track, error) {
	return *lib, nil
}

// TrackInfo implements the library.Library interface.
func (lib *DummyLibrary) TrackInfo(ctx context.Context, uris ...string) ([]Track, error) {
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
func (lib *DummyLibrary) TrackArt(ctx context.Context, uri string) (io.ReadCloser, string, error) {
	return nil, "", ErrNoArt
}

// Events implements the player.Player interface.
//
// DummyLibrary is stateless, so a dummy Emitter is returned.
func (lib *DummyLibrary) Events() *util.Emitter {
	return &util.Emitter{}
}
