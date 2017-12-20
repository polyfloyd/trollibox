package slimserver

import (
	"fmt"
	"testing"

	"github.com/polyfloyd/trollibox/src/player"
)

func connectForTesting() (*Player, error) {
	server, err := Connect("tcp", "127.0.0.1:9090", nil, nil, "http://127.0.0.1:9000/")
	if err != nil {
		return nil, err
	}
	players, err := server.Players()
	if err != nil {
		return nil, err
	}
	if len(players) == 0 {
		return nil, fmt.Errorf("the SlimServer is not connected to any players")
	}
	return players[0], nil
}

func TestPlayerImplementation(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Skipf("%v", err)
	}
	player.TestPlayerImplementation(t, pl)
}

func TestPlaylistImplementation(t *testing.T) {
	pl, err := connectForTesting()
	if err != nil {
		t.Skipf("%v", err)
	}
	tracks, err := pl.Tracks()
	if err != nil {
		t.Fatal(err)
	}
	player.TestPlaylistImplementation(t, pl.Playlist(), tracks[:3])
}
