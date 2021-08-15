package player

import (
	"context"
	"sort"
	"testing"

	"trollibox/src/library"
)

// TestPlaylistImplementation tests the implementation of the playerPlaylist interface.
func TestPlaylistImplementation(t *testing.T, ls Playlist, testTracks []library.Track) {
	ctx := context.Background()
	clear := func() {
		if length, err := ls.Len(ctx); err != nil {
			t.Fatal(err)
		} else {
			rm := make([]int, length)
			for i := range rm {
				rm[i] = i
			}
			if err := ls.Remove(ctx, rm...); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Run("len", func(t *testing.T) {
		clear()
		testPlaylistLen(ctx, t, ls, testTracks)
	})
	t.Run("insert", func(t *testing.T) {
		clear()
		testPlaylistInsert(ctx, t, ls, testTracks)
	})
	t.Run("append", func(t *testing.T) {
		clear()
		testPlaylistAppend(ctx, t, ls, testTracks)
	})
	t.Run("move", func(t *testing.T) {
		clear()
		testPlaylistMove(ctx, t, ls, testTracks)
	})
	t.Run("remove", func(t *testing.T) {
		clear()
		testPlaylistRemove(ctx, t, ls, testTracks)
	})
}

func testPlaylistLen(ctx context.Context, t *testing.T, ls Playlist, testTracks []library.Track) {
	if l, err := ls.Len(ctx); err != nil {
		t.Fatal(err)
	} else if l != 0 {
		t.Fatalf("Initial length is not 0, got %d", l)
	}
	if err := ls.Insert(ctx, -1, testTracks...); err != nil {
		t.Fatal(err)
	}
	if l, err := ls.Len(ctx); err != nil {
		t.Fatal(err)
	} else if l != len(testTracks) {
		t.Fatalf("Inserted track count mismatch: %d != %d", len(testTracks), l)
	}
}

func testPlaylistInsert(ctx context.Context, t *testing.T, ls Playlist, testTracks []library.Track) {
	if err := ls.Insert(ctx, 0, testTracks[1:]...); err != nil {
		t.Fatal(err)
	}

	tracks, err := ls.Tracks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, testTrack := range testTracks[1:] {
		if testTrack.URI != tracks[i].URI {
			t.Logf("expected: %v", testTracks[1:])
			t.Logf("got: %v", tracks)
			t.Fatalf("Mismatched tracks at index %d", i)
		}
	}

	if err := ls.Insert(ctx, 0, testTracks[0]); err != nil {
		t.Fatal(err)
	}
	if tracks, err = ls.Tracks(ctx); err != nil {
		t.Fatal(err)
	} else if tracks[0].URI != testTracks[0].URI {
		t.Logf("expected %q at index 0", testTracks[0].URI)
		t.Logf("got: %v", tracks)
		t.Fatalf("Insert error: %q not inserted at position 0", testTracks[0].URI)
	}
}

func testPlaylistAppend(ctx context.Context, t *testing.T, ls Playlist, testTracks []library.Track) {
	if err := ls.Insert(ctx, 0, testTracks[1:]...); err != nil {
		t.Fatal(err)
	}
	if err := ls.Insert(ctx, -1, testTracks[0]); err != nil {
		t.Fatal(err)
	}
	tracks, err := ls.Tracks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tracks[len(tracks)-1].URI != testTracks[0].URI {
		t.Fatalf("Insert error: track not appended")
	}
}

func testPlaylistMove(ctx context.Context, t *testing.T, ls Playlist, testTracks []library.Track) {
	if err := ls.Insert(ctx, -1, testTracks...); err != nil {
		t.Fatal(err)
	}
	tracksBefore, _ := ls.Tracks(ctx)
	if err := ls.Move(ctx, 0, 1); err != nil {
		t.Fatal(err)
	}
	if tracks, err := ls.Tracks(ctx); err != nil {
		t.Fatal(err)
	} else if tracks[1].URI != testTracks[0].URI {
		t.Logf("Tracks before:")
		for _, track := range tracksBefore {
			t.Logf("  %s", track.URI)
		}
		t.Logf("Tracks after:")
		for _, track := range tracks {
			t.Logf("  %s", track.URI)
		}
		t.Fatalf("Track was not moved or moved to the wrong index")
	}
}

func testPlaylistRemove(ctx context.Context, t *testing.T, ls Playlist, testTracks []library.Track) {
	if err := ls.Insert(ctx, -1, testTracks...); err != nil {
		t.Fatal(err)
	}
	indices := make([]int, len(testTracks))
	for i := 0; i < len(indices); i++ {
		indices[i] = i
	}
	if err := ls.Remove(ctx, indices...); err != nil {
		t.Fatal(err)
	}
	if l, err := ls.Len(ctx); err != nil {
		t.Fatal(err)
	} else if l != 0 {
		t.Fatalf("Not all tracks were removed: %d remaining", l)
	}
}

// DummyPlaylist is used for testing.
type DummyPlaylist []library.Track

// Insert implements the player.Playlist interface.
func (pl *DummyPlaylist) Insert(ctx context.Context, pos int, tracks ...library.Track) error {
	if pos == -1 {
		pos, _ = pl.Len(ctx)
	}
	*pl = append(append((*pl)[:pos], tracks...), (*pl)[pos:]...)
	return nil
}

// Move implements the player.Playlist interface.
func (pl *DummyPlaylist) Move(ctx context.Context, fromPos, toPos int) error {
	moved := (*pl)[fromPos]
	cut := append((*pl)[:fromPos], (*pl)[fromPos+1:]...)
	delta := 0
	if fromPos > toPos {
		delta = -1
	}
	*pl = append(append(cut[:toPos+delta], moved), (*pl)[toPos+1+delta:]...)
	return nil
}

// Remove implements the player.Playlist interface.
func (pl *DummyPlaylist) Remove(ctx context.Context, pos ...int) error {
	sort.Ints(pos)
	for i, p := range pos {
		*pl = append((*pl)[:p-i], (*pl)[p+1-i:]...)
	}
	return nil
}

// Tracks implements the player.Playlist interface.
func (pl *DummyPlaylist) Tracks(ctx context.Context) ([]library.Track, error) {
	return *pl, nil
}

// Len implements the player.Playlist interface.
func (pl *DummyPlaylist) Len(ctx context.Context) (int, error) {
	return len(*pl), nil
}
