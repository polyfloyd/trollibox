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
		t.Fatal(err)
	}
	player.TestPlayerImplementation(t, pl)
}

func TestPlaylistImplementation(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Fatal(err)
	}
	tracks, err := pl.Tracks()
	if err != nil {
		t.Fatal(err)
	}
	player.TestPlaylistImplementation(t, pl.Playlist(), tracks[:3])
}
