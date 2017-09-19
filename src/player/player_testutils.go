package player

import (
	"fmt"
	"time"
)

func fillPlaylist(pl Player, numTracks int) error {
	tracks, err := pl.Tracks()
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
		return fmt.Errorf("Not enough tracks in the library: %v < %v", len(tracks), numTracks)
	}
	return pl.Playlist().Insert(0, tracks[0:numTracks]...)
}

func testEvent(pl Player, event string, cb func() error) error {
	if err := fillPlaylist(pl, 2); err != nil {
		return err
	}
	l := pl.Events().Listen()
	defer pl.Events().Unlisten(l)
	if err := cb(); err != nil {
		return err
	}
	for {
		select {
		case msg := <-l:
			if msg == event {
				return nil
			}
		case <-time.After(time.Second):
			return fmt.Errorf("Event %q was not emitted", event)
		}
	}
}

func TestTrackIndex(pl Player) error {
	if err := fillPlaylist(pl, 2); err != nil {
		return err
	}
	if err := pl.SetTrackIndex(0); err != nil {
		return err
	}
	if index, err := pl.TrackIndex(); err != nil {
		return err
	} else if index != 0 {
		return fmt.Errorf("Unexpected track index: %v != %v", 0, index)
	}
	if state, err := pl.State(); err != nil {
		return err
	} else if state != PlayStatePlaying {
		return fmt.Errorf("Unexpected state: %v", state)
	}

	if err := pl.SetTrackIndex(1); err != nil {
		return err
	}
	if index, err := pl.TrackIndex(); err != nil {
		return err
	} else if index != 1 {
		return fmt.Errorf("Unexpected track index: %v != %v", 1, index)
	}
	return nil
}

func TestTrackIndexEvent(pl Player) error {
	if err := fillPlaylist(pl, 2); err != nil {
		return err
	}
	return testEvent(pl, "playlist", func() error {
		return pl.SetTrackIndex(1)
	})
}

func TestPlaystate(pl Player) error {
	if err := fillPlaylist(pl, 1); err != nil {
		return err
	}

	if err := pl.SetState(PlayStatePlaying); err != nil {
		return err
	}
	if state, err := pl.State(); err != nil {
		return err
	} else if state != PlayStatePlaying {
		return fmt.Errorf("Unexpected state: %v", state)
	}

	if err := pl.SetState(PlayStatePaused); err != nil {
		return err
	}
	if state, err := pl.State(); err != nil {
		return err
	} else if state != PlayStatePaused {
		return fmt.Errorf("Unexpected state: %v", state)
	}

	if err := pl.SetState(PlayStateStopped); err != nil {
		return err
	}
	if state, err := pl.State(); err != nil {
		return err
	} else if state != PlayStateStopped {
		return fmt.Errorf("Unexpected state: %v", state)
	}
	return nil
}

func TestPlaystateEvent(pl Player) error {
	if err := fillPlaylist(pl, 1); err != nil {
		return err
	}
	return testEvent(pl, "playstate", func() error {
		if err := pl.SetState(PlayStatePlaying); err != nil {
			return err
		}
		return pl.SetState(PlayStateStopped)
	})
}

func TestVolume(pl Player) error {
	const VOL_A = 0.2
	const VOL_B = 0.4
	if err := fillPlaylist(pl, 1); err != nil {
		return err
	}

	if err := pl.SetState(PlayStatePlaying); err != nil {
		return err
	}
	if err := pl.SetVolume(VOL_A); err != nil {
		return err
	}
	if vol, err := pl.Volume(); err != nil {
		return err
	} else if vol != VOL_A {
		return fmt.Errorf("Volume does not match expected value, %v != %v", VOL_A, vol)
	}

	if err := pl.SetState(PlayStateStopped); err != nil {
		return err
	}
	if err := pl.SetVolume(VOL_B); err != nil {
		return err
	}
	if vol, err := pl.Volume(); err != nil {
		return err
	} else if vol != VOL_B {
		return fmt.Errorf("Volume does not match expected value, %v != %v", VOL_B, vol)
	}
	return nil
}

func TestVolumeEvent(pl Player) error {
	if err := fillPlaylist(pl, 1); err != nil {
		return err
	}
	return testEvent(pl, "volume", func() error {
		return pl.SetVolume(0.2)
	})
}
