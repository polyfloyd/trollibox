package player

import (
	"testing"

	"trollibox/src/library"
)

func TestDummyPlaylistImplementation(t *testing.T) {
	tracks := []library.Track{
		{
			URI:    "track1",
			Artist: "Artist 1",
			Title:  "Title 1",
		},
		{
			URI:    "track2",
			Artist: "Artist 2",
			Title:  "Title 2",
		},
		{
			URI:    "track3",
			Artist: "Artist 3",
			Title:  "Title 3",
		},
	}
	TestPlaylistImplementation(t, &DummyPlaylist{}, tracks)
}
