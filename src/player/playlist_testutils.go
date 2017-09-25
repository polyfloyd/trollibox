package player

import (
	"testing"
)

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
	if err := ls.Insert(0, testTracks...); err != nil {
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
	for i := 1; i < len(testTracks); i++ {
		if testTracks[i].Uri != tracks[i-1].Uri {
			t.Fatalf("Mismatched tracks at index %d", i)
		}
	}

	if err := ls.Insert(0, testTracks[0]); err != nil {
		t.Fatal(err)
	}
	tracks, err = ls.Tracks()
	if err != nil {
		t.Fatal(err)
	}
	if tracks[0].Uri != testTracks[0].Uri {
		t.Fatalf("Insert error: track not inserted at position 0")
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
	if tracks[len(tracks)-1].Uri != testTracks[0].Uri {
		t.Fatalf("Insert error: track not appended")
	}
}

func testPlaylistMove(t *testing.T, ls Playlist, testTracks []Track) {
	if err := ls.Insert(0, testTracks...); err != nil {
		t.Fatal(err)
	}
	if err := ls.Move(0, 1); err != nil {
		t.Fatal(err)
	}
	if tracks, err := ls.Tracks(); err != nil {
		t.Fatal(err)
	} else if tracks[1].Uri != testTracks[0].Uri {
		t.Fatalf("Track was not moved")
	}
}

func testPlaylistRemove(t *testing.T, ls Playlist, testTracks []Track) {
	if err := ls.Insert(0, testTracks...); err != nil {
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
