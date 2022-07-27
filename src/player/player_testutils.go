package player

import (
	"context"
	"fmt"
	"testing"
	"time"

	"trollibox/src/util"
)

func fillPlaylist(ctx context.Context, pl Player, numTracks int) error {
	tracks, err := pl.Library().Tracks(ctx)
	if err != nil {
		return err
	}
	length, err := pl.Playlist().Len(ctx)
	if err != nil {
		return err
	}
	rm := make([]int, length)
	for i := range rm {
		rm[i] = i
	}
	if err := pl.Playlist().Remove(ctx, rm...); err != nil {
		return err
	}
	if len(tracks) < numTracks {
		return fmt.Errorf("not enough tracks in the library: %v < %v", len(tracks), numTracks)
	}

	metaTracks := make([]MetaTrack, numTracks)
	for i, t := range tracks[0:numTracks] {
		metaTracks[i] = MetaTrack{
			Track:    t,
			QueuedBy: "system",
		}
	}
	return pl.Playlist().Insert(ctx, 0, metaTracks...)
}

// TestPlayerImplementation tests the implementation of the player.Player interface.
func TestPlayerImplementation(t *testing.T, pl Player) {
	ctx := context.Background()
	if err := fillPlaylist(ctx, pl, 3); err != nil {
		t.Fatal(err)
	}
	t.Run("time", func(t *testing.T) {
		testTime(ctx, t, pl)
	})
	t.Run("time_event", func(t *testing.T) {
		testTimeEvent(ctx, t, pl)
	})
	t.Run("trackindex", func(t *testing.T) {
		testTrackIndex(ctx, t, pl)
	})
	t.Run("trackindex_event", func(t *testing.T) {
		testTrackIndexEvent(ctx, t, pl)
	})
	t.Run("playstate", func(t *testing.T) {
		testPlayState(ctx, t, pl)
	})
	t.Run("playstate_event", func(t *testing.T) {
		testPlayStateEvent(ctx, t, pl)
	})
	t.Run("volume", func(t *testing.T) {
		testVolume(ctx, t, pl)
	})
	t.Run("volume_event", func(t *testing.T) {
		testVolumeEvent(ctx, t, pl)
	})
}

func testTime(ctx context.Context, t *testing.T, pl Player) {
	const timeA = time.Second * 2
	if err := pl.SetState(ctx, PlayStatePlaying); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetState(ctx, PlayStatePaused); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetTime(ctx, timeA); err != nil {
		t.Fatal(err)
	}
	if status, err := pl.Status(ctx); err != nil {
		t.Fatal(err)
	} else if status.Time != timeA {
		t.Fatalf("Unexpected time: %v != %v", timeA, status.Time)
	}
}

func testTimeEvent(ctx context.Context, t *testing.T, pl Player) {
	newTime := time.Second * 2
	util.TestEventEmission(t, pl, TimeEvent{Time: newTime}, func() {
		if err := pl.SetState(ctx, PlayStatePlaying); err != nil {
			t.Fatal(err)
		}
		if err := pl.SetTime(ctx, newTime); err != nil {
			t.Fatal(err)
		}
	})
}

func testTrackIndex(ctx context.Context, t *testing.T, pl Player) {
	if err := pl.SetTrackIndex(ctx, 0); err != nil {
		t.Fatal(err)
	}

	status, err := pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.TrackIndex != 0 {
		t.Fatalf("Unexpected track index: %v != %v", 0, status.TrackIndex)
	}
	if status.PlayState != PlayStatePlaying {
		t.Fatalf("Unexpected state: %v", status.PlayState)
	}

	if err := pl.SetTrackIndex(ctx, 1); err != nil {
		t.Fatal(err)
	}
	status, err = pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.TrackIndex != 1 {
		t.Fatalf("Unexpected track index: %v != %v", 1, status.TrackIndex)
	}

	if err := pl.SetTrackIndex(ctx, 99); err != nil {
		t.Fatal(err)
	}
	status, err = pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.PlayState != PlayStateStopped {
		t.Fatalf("Unexpected state: %v", status.PlayState)
	}
}

func testTrackIndexEvent(ctx context.Context, t *testing.T, pl Player) {
	util.TestEventEmission(t, pl, PlaylistEvent{TrackIndex: 1}, func() {
		if err := pl.SetTrackIndex(ctx, 1); err != nil {
			t.Fatal(err)
		}
	})
}

func testPlayState(ctx context.Context, t *testing.T, pl Player) {
	if err := pl.SetState(ctx, PlayStatePlaying); err != nil {
		t.Fatal(err)
	}
	status, err := pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.PlayState != PlayStatePlaying {
		t.Fatalf("Unexpected state: %v", status.PlayState)
	}

	if err := pl.SetState(ctx, PlayStatePaused); err != nil {
		t.Fatal(err)
	}
	status, err = pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.PlayState != PlayStatePaused {
		t.Fatalf("Unexpected state: %v", status.PlayState)
	}

	if err := pl.SetState(ctx, PlayStateStopped); err != nil {
		t.Fatal(err)
	}
	status, err = pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.PlayState != PlayStateStopped {
		t.Fatalf("Unexpected state: %v", status.PlayState)
	}
}

func testPlayStateEvent(ctx context.Context, t *testing.T, pl Player) {
	if err := pl.SetState(ctx, PlayStatePlaying); err != nil {
		t.Fatal(err)
	}
	util.TestEventEmission(t, pl, PlayStateEvent{State: PlayStateStopped}, func() {
		if err := pl.SetState(ctx, PlayStateStopped); err != nil {
			t.Fatal(err)
		}
	})
}

func testVolume(ctx context.Context, t *testing.T, pl Player) {
	const volA = 20
	const volB = 40
	if err := pl.SetState(ctx, PlayStatePlaying); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetVolume(ctx, volA); err != nil {
		t.Fatal(err)
	}
	status, err := pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Volume != volA {
		t.Fatalf("Volume does not match expected value, %v != %v", volA, status.Volume)
	}

	if err := pl.SetState(ctx, PlayStateStopped); err != nil {
		t.Fatal(err)
	}
	if err := pl.SetVolume(ctx, volB); err != nil {
		t.Fatal(err)
	}
	status, err = pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Volume != volB {
		t.Fatalf("Volume does not match expected value, %v != %v", volB, status.Volume)
	}

	if err := pl.SetVolume(ctx, 200); err != nil {
		t.Fatal(err)
	}
	status, err = pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Volume != 100 {
		t.Fatalf("Volume was not clamped: %v", status.Volume)
	}

	if err := pl.SetVolume(ctx, -100); err != nil {
		t.Fatal(err)
	}
	status, err = pl.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Volume != 0 {
		t.Fatalf("Volume was not clamped: %v", status.Volume)
	}
}

func testVolumeEvent(ctx context.Context, t *testing.T, pl Player) {
	if err := pl.SetVolume(ctx, 40); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	util.TestEventEmission(t, pl, VolumeEvent{Volume: 20}, func() {
		if err := pl.SetVolume(ctx, 20); err != nil {
			t.Fatal(err)
		}
	})
}
