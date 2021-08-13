package player

import (
	"testing"

	"trollibox/src/library"
)

func TestMetaKeeperPlaylistImplementation(t *testing.T) {
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
	metapl := &PlaylistMetaKeeper{Playlist: &DummyPlaylist{}}
	TestPlaylistImplementation(t, metapl, tracks)
}

func TestMetaKeeperInsert(t *testing.T) {
	metapl := PlaylistMetaKeeper{Playlist: &DummyPlaylist{}}
	if err := metapl.InsertWithMeta(0, []library.Track{{}, {}}, []TrackMeta{{}}); err == nil {
		t.Fatalf("The Metakeeper should not accept track and meta slices which lengths do not match")
	}

	if err := metapl.InsertWithMeta(0, []library.Track{{}}, []TrackMeta{{QueuedBy: "system"}}); err != nil {
		t.Fatal(err)
	}
	tracks, err := metapl.MetaTracks()
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
