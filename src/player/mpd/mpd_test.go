package mpd

import (
	"testing"

	player ".."
)

func connectForTesting() (*Player, error) {
	return Connect("tcp", "127.0.0.1:6600", nil)
}

func TestPlayerImplementation(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Error(err)
		return
	}
	player.TestPlayerImplementation(t, pl)
}
