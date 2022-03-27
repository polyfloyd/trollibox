package player

import (
	"context"
	"testing"

	"trollibox/src/library"
)

func TestMetaKeeperPlaylistImplementation(t *testing.T) {
	tracks := []MetaTrack{
		{
			Track: library.Track{
				URI:    "track1",
				Artist: "Artist 1",
				Title:  "Title 1",
			},
			TrackMeta: TrackMeta{QueuedBy: "system"},
		},
		{
			Track: library.Track{
				URI:    "track2",
				Artist: "Artist 2",
				Title:  "Title 2",
			},
			TrackMeta: TrackMeta{QueuedBy: "system"},
		},
		{
			Track: library.Track{
				URI:    "track3",
				Artist: "Artist 3",
				Title:  "Title 3",
			},
			TrackMeta: TrackMeta{QueuedBy: "system"},
		},
	}
	metapl := &PlaylistMetaKeeper{Playlist: &DummyPlaylist{}}
	TestPlaylistImplementation[MetaTrack](t, metapl, tracks)
}

func TestMetaKeeperInsert(t *testing.T) {
	ctx := context.Background()

	metapl := PlaylistMetaKeeper{Playlist: &DummyPlaylist{}}
	if err := metapl.Insert(ctx, 0, MetaTrack{TrackMeta: TrackMeta{QueuedBy: "system"}}); err != nil {
		t.Fatal(err)
	}
	tracks, err := metapl.Tracks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 {
		t.Fatalf("Unexpected metadata length: %v", len(tracks))
	}
	if tracks[0].QueuedBy != "system" {
		t.Fatalf("Unexpected QueuedBy: %v", tracks[0].QueuedBy)
	}
}
