package mpd

import (
	"testing"

	player ".."
)

func connectForTesting() (*Player, error) {
	return Connect("tcp", "127.0.0.1:6600", nil)
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
