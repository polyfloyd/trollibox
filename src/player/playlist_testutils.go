package player

import (
	"sort"
	"testing"
)

// TestPlaylistImplementation tests the implementation of the playerPlaylist interface.
func TestPlaylistImplementation(t *testing.T, ls Playlist, testTracks []Track) {
	clear := func() {
		if length, err := ls.Len(); err != nil {
			t.Fatal(err)
		} else {
			rm := make([]int, length)
			for i := range rm {
				rm[i] = i
			}
			if err := ls.Remove(rm...); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Run("len", func(t *testing.T) {
		clear()
		testPlaylistLen(t, ls, testTracks)
	})
	t.Run("insert", func(t *testing.T) {
		clear()
		testPlaylistInsert(t, ls, testTracks)
	})
	t.Run("append", func(t *testing.T) {
		clear()
		testPlaylistAppend(t, ls, testTracks)
	})
	t.Run("move", func(t *testing.T) {
		clear()
		testPlaylistMove(t, ls, testTracks)
	})
	t.Run("remove", func(t *testing.T) {
		clear()
		testPlaylistRemove(t, ls, testTracks)
	})
}

func testPlaylistLen(t *testing.T, ls Playlist, testTracks []Track) {
	if l, err := ls.Len(); err != nil {
		t.Fatal(err)
	} else if l != 0 {
		t.Fatalf("Initial length is not 0, got %d", l)
	}
	if err := ls.Insert(-1, testTracks...); err != nil {
		t.Fatal(err)
	}
	if l, err := ls.Len(); err != nil {
		t.Fatal(err)
	} else if l != len(testTracks) {
		t.Fatalf("Inserted track count mismatch: %d != %d", len(testTracks), l)
	}
}

func testPlaylistInsert(t *testing.T, ls Playlist, testTracks []Track) {
	if err := ls.Insert(0, testTracks[1:]...); err != nil {
		t.Fatal(err)
	}

	tracks, err := ls.Tracks()
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

	if err := ls.Insert(0, testTracks[0]); err != nil {
		t.Fatal(err)
	}
	if tracks, err = ls.Tracks(); err != nil {
		t.Fatal(err)
	} else if tracks[0].URI != testTracks[0].URI {
		t.Logf("expected %q at index 0", testTracks[0].URI)
		t.Logf("got: %v", tracks)
		t.Fatalf("Insert error: %q not inserted at position 0", testTracks[0].URI)
	}
}

func testPlaylistAppend(t *testing.T, ls Playlist, testTracks []Track) {
	if err := ls.Insert(0, testTracks[1:]...); err != nil {
		t.Fatal(err)
	}
	if err := ls.Insert(-1, testTracks[0]); err != nil {
		t.Fatal(err)
	}
	tracks, err := ls.Tracks()
	if err != nil {
		t.Fatal(err)
	}
	if tracks[len(tracks)-1].URI != testTracks[0].URI {
		t.Fatalf("Insert error: track not appended")
	}
}

func testPlaylistMove(t *testing.T, ls Playlist, testTracks []Track) {
	if err := ls.Insert(-1, testTracks...); err != nil {
		t.Fatal(err)
	}
	tracksBefore, _ := ls.Tracks()
	if err := ls.Move(0, 1); err != nil {
		t.Fatal(err)
	}
	if tracks, err := ls.Tracks(); err != nil {
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

func testPlaylistRemove(t *testing.T, ls Playlist, testTracks []Track) {
	if err := ls.Insert(-1, testTracks...); err != nil {
		t.Fatal(err)
	}
	indices := make([]int, len(testTracks))
	for i := 0; i < len(indices); i++ {
		indices[i] = i
	}
	if err := ls.Remove(indices...); err != nil {
		t.Fatal(err)
	}
	if l, err := ls.Len(); err != nil {
		t.Fatal(err)
	} else if l != 0 {
		t.Fatalf("Not all tracks were removed: %d remaining", l)
	}
}

// DummyPlaylist is used for testing.
type DummyPlaylist []Track

// Insert implements the player.Playlist interface.
func (pl *DummyPlaylist) Insert(pos int, tracks ...Track) error {
	if pos == -1 {
		pos, _ = pl.Len()
	}
	*pl = append(append((*pl)[:pos], tracks...), (*pl)[pos:]...)
	return nil
}

// Move implements the player.Playlist interface.
func (pl *DummyPlaylist) Move(fromPos, toPos int) error {
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
func (pl *DummyPlaylist) Remove(pos ...int) error {
	sort.Ints(pos)
	for i, p := range pos {
		*pl = append((*pl)[:p-i], (*pl)[p+1-i:]...)
	}
	return nil
}

// Tracks implements the player.Playlist interface.
func (pl *DummyPlaylist) Tracks() ([]Track, error) {
	return *pl, nil
}

// Len implements the player.Playlist interface.
func (pl *DummyPlaylist) Len() (int, error) {
	return len(*pl), nil
}
