package player

import (
	"context"
	"reflect"
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
			QueuedBy: "system",
		},
		{
			Track: library.Track{
				URI:    "track2",
				Artist: "Artist 2",
				Title:  "Title 2",
			},
			QueuedBy: "system",
		},
		{
			Track: library.Track{
				URI:    "track3",
				Artist: "Artist 3",
				Title:  "Title 3",
			},
			QueuedBy: "system",
		},
	}
	metapl := &PlaylistMetaKeeper{Playlist: &DummyPlaylist{}}
	TestPlaylistImplementation[MetaTrack](t, metapl, tracks)
}

func TestMetaKeeperListTracks(t *testing.T) {
	ctx := context.Background()
	tracks := []MetaTrack{
		{
			Track: library.Track{
				URI:    "track1",
				Artist: "Artist 1",
				Title:  "Title 1",
			},
			QueuedBy: "foo",
		},
		{
			Track: library.Track{
				URI:    "track2",
				Artist: "Artist 2",
				Title:  "Title 2",
			},
			QueuedBy: "bar",
		},
		{
			Track: library.Track{
				URI:    "track3",
				Artist: "Artist 3",
				Title:  "Title 3",
			},
			QueuedBy: "baz",
		},
	}
	metapl := &PlaylistMetaKeeper{Playlist: &DummyPlaylist{}}
	if err := metapl.Insert(ctx, 0, tracks...); err != nil {
		t.Fatal(err)
	}

	queuedBys := func() []string {
		tracks, err := metapl.Tracks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		qq := make([]string, len(tracks))
		for i, t := range tracks {
			qq[i] = t.QueuedBy
		}
		return qq
	}

	if qq := queuedBys(); !reflect.DeepEqual(qq, []string{"foo", "bar", "baz"}) {
		t.Fatalf("Unexpected QueuedBy's: %q", qq)
	}
	if err := metapl.Insert(ctx, -1, MetaTrack{Track: tracks[0].Track, QueuedBy: "foo"}); err != nil {
		t.Fatal(err)
	}
	if qq := queuedBys(); !reflect.DeepEqual(qq, []string{"foo", "bar", "baz", "foo"}) {
		t.Fatalf("Unexpected QueuedBy's: %q", qq)
	}
	if err := metapl.Insert(ctx, 0, MetaTrack{Track: tracks[0].Track, QueuedBy: "qux"}); err != nil {
		t.Fatal(err)
	}
	if qq := queuedBys(); !reflect.DeepEqual(qq, []string{"qux", "foo", "bar", "baz", "foo"}) {
		t.Fatalf("Unexpected QueuedBy's: %q", qq)
	}
}

func TestMetaKeeperInsert(t *testing.T) {
	ctx := context.Background()

	metapl := PlaylistMetaKeeper{Playlist: &DummyPlaylist{}}
	if err := metapl.Insert(ctx, 0, MetaTrack{QueuedBy: "system"}); err != nil {
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
