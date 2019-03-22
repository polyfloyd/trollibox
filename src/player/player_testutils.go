package player

import (
	"fmt"
	"testing"
	"time"

	"github.com/polyfloyd/trollibox/src/util"
)

func fillPlaylist(pl Player, numTracks int) error {
	tracks, err := pl.Library().Tracks()
	if err != nil {
		return err
	}
	length, err := pl.Playlist().Len()
	if err != nil {
		return err
	}
	rm := make([]int, length)
	for i := range rm {
		rm[i] = i
	}
	if err := pl.Playlist().Remove(rm...); err != nil {
		return err
	}
	if len(tracks) < numTracks {
		return fmt.Errorf("not enough tracks in the library: %v < %v", len(tracks), numTracks)
	}
	return pl.Playlist().Insert(0, tracks[0:numTracks]...)
}

// TestPlayerImplementation tests the implementation of the player.Player interface.
func TestPlayerImplementation(t *testing.T, pl Player) {
	if err := fillPlaylist(pl, 3); err != nil {
		t.Fatal(err)
	}
	t.Run("availability", func(t *testing.T) {
		testAvailability(t, pl)
	})
	t.Run("time", func(t *testing.T) {
		testTime(t, pl)
	})
	t.Run("time_event", func(t *testing.T) {
		testTimeEvent(t, pl)
	})
	t.Run("trackindex", func(t *testing.T) {
		testTrackIndex(t, pl)
	})
	t.Run("trackindex_event", func(t *testing.T) {
		testTrackIndexEvent(t, pl)
	})
	t.Run("playstate", func(t *testing.T) {
		testPlaystate(t, pl)
	})
	t.Run("playstate_event", func(t *testing.T) {
		testPlaystateEvent(t, pl)
	})
	t.Run("volume", func(t *testing.T) {
		testVolume(t, pl)
	})
	t.Run("volume_event", func(t *testing.T) {
		testVolumeEvent(t, pl)
	})
}

func testAvailability(t *testing.T, pl Player) {
	if !pl.Available() {
		t.Fatal("The player is not available")
	}
}

func testTime(t *testing.T, pl Player) {
	const timeA = time.Second * 2
	if err := pl.SetState(PlayStatePlaying); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetState(PlayStatePaused); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetTime(timeA); err != nil {
		t.Fatal(err)
	}
	if curTime, err := pl.Time(); err != nil {
		t.Fatal(err)
	} else if curTime != timeA {
		t.Fatalf("Unexpected time: %v != %v", timeA, curTime)
	}
}

func testTimeEvent(t *testing.T, pl Player) {
	util.TestEventEmission(t, pl, TimeEvent, func() {
		if err := pl.SetState(PlayStatePlaying); err != nil {
			t.Fatal(err)
		}
		if err := pl.SetTime(time.Second * 2); err != nil {
			t.Fatal(err)
		}
	})
}

func testTrackIndex(t *testing.T, pl Player) {
	if err := pl.SetTrackIndex(0); err != nil {
		t.Fatal(err)
	}
	if index, err := pl.TrackIndex(); err != nil {
		t.Fatal(err)
	} else if index != 0 {
		t.Fatalf("Unexpected track index: %v != %v", 0, index)
	}
	if state, err := pl.State(); err != nil {
		t.Fatal(err)
	} else if state != PlayStatePlaying {
		t.Fatalf("Unexpected state: %v", state)
	}

	if err := pl.SetTrackIndex(1); err != nil {
		t.Fatal(err)
	}
	if index, err := pl.TrackIndex(); err != nil {
		t.Fatal(err)
	} else if index != 1 {
		t.Fatalf("Unexpected track index: %v != %v", 1, index)
	}

	if err := pl.SetTrackIndex(99); err != nil {
		t.Fatal(err)
	}
	if state, err := pl.State(); err != nil {
		t.Fatal(err)
	} else if state != PlayStateStopped {
		t.Fatalf("Unexpected state: %v", state)
	}
}

func testTrackIndexEvent(t *testing.T, pl Player) {
	util.TestEventEmission(t, pl, PlaylistEvent, func() {
		if err := pl.SetTrackIndex(1); err != nil {
			t.Fatal(err)
		}
	})
}

func testPlaystate(t *testing.T, pl Player) {
	if err := pl.SetState(PlayStatePlaying); err != nil {
		t.Fatal(err)
	}
	if state, err := pl.State(); err != nil {
		t.Fatal(err)
	} else if state != PlayStatePlaying {
		t.Fatalf("Unexpected state: %v", state)
	}

	if err := pl.SetState(PlayStatePaused); err != nil {
		t.Fatal(err)
	}
	if state, err := pl.State(); err != nil {
		t.Fatal(err)
	} else if state != PlayStatePaused {
		t.Fatalf("Unexpected state: %v", state)
	}

	if err := pl.SetState(PlayStateStopped); err != nil {
		t.Fatal(err)
	}
	if state, err := pl.State(); err != nil {
		t.Fatal(err)
	} else if state != PlayStateStopped {
		t.Fatalf("Unexpected state: %v", state)
	}
}

func testPlaystateEvent(t *testing.T, pl Player) {
	util.TestEventEmission(t, pl, PlaystateEvent, func() {
		if err := pl.SetState(PlayStatePlaying); err != nil {
			t.Fatal(err)
		}
		if err := pl.SetState(PlayStateStopped); err != nil {
			t.Fatal(err)
		}
	})
}

func testVolume(t *testing.T, pl Player) {
	const volA = 20
	const volB = 40
	if err := pl.SetState(PlayStatePlaying); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetVolume(volA); err != nil {
		t.Fatal(err)
	}
	if vol, err := pl.Volume(); err != nil {
		t.Fatal(err)
	} else if vol != volA {
		t.Fatalf("Volume does not match expected value, %v != %v", volA, vol)
	}

	if err := pl.SetState(PlayStateStopped); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetVolume(volB); err != nil {
		t.Fatal(err)
	}
	if vol, err := pl.Volume(); err != nil {
		t.Fatal(err)
	} else if vol != volB {
		t.Fatalf("Volume does not match expected value, %v != %v", volB, vol)
	}

	if err := pl.SetVolume(200); err != nil {
		t.Fatal(err)
	}
	if vol, err := pl.Volume(); err != nil {
		t.Fatal(err)
	} else if vol != 100 {
		t.Fatalf("Volume was not clamped: %v", vol)
	}

	if err := pl.SetVolume(-100); err != nil {
		t.Fatal(err)
	}
	if vol, err := pl.Volume(); err != nil {
		t.Fatal(err)
	} else if vol != 0 {
		t.Fatalf("Volume was not clamped: %v", vol)
	}
}

func testVolumeEvent(t *testing.T, pl Player) {
	if err := pl.SetVolume(40); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	util.TestEventEmission(t, pl, VolumeEvent, func() {
		if err := pl.SetVolume(20); err != nil {
			t.Fatal(err)
		}
	})
}
