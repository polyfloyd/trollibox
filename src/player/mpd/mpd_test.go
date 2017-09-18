package mpd

import (
	"testing"

	player ".."
)

func connectForTesting() (*Player, error) {
	return Connect("tcp", "127.0.0.1:6600", nil)
}

func TestTrackIndex(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Error(err)
		return
	}
	if err := player.TestTrackIndex(pl); err != nil {
		t.Error(err)
		return
	}
}

func TestTrackIndexEvent(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Error(err)
		return
	}
	if err := player.TestTrackIndexEvent(pl); err != nil {
		t.Error(err)
		return
	}
}

func TestPlaystate(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Error(err)
		return
	}
	if err := player.TestPlaystate(pl); err != nil {
		t.Error(err)
		return
	}
}

func TestPlaystateEvent(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Error(err)
		return
	}
	if err := player.TestPlaystateEvent(pl); err != nil {
		t.Error(err)
		return
	}
}

func TestVolume(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Error(err)
		return
	}
	if err := player.TestVolume(pl); err != nil {
		t.Error(err)
		return
	}
}

func TestVolumeEvent(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Error(err)
		return
	}
	if err := player.TestVolumeEvent(pl); err != nil {
		t.Error(err)
		return
	}
}
