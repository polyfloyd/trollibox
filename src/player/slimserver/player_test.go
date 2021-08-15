package slimserver

import (
	"context"
	"fmt"
	"testing"

	"trollibox/src/player"
)

func connectForTesting() (*Player, error) {
	server, err := Connect("tcp", "127.0.0.1:9090", nil, nil, "http://127.0.0.1:9000/")
	if err != nil {
		return nil, err
	}
	playerNames, err := server.PlayerNames()
	if err != nil {
		return nil, err
	}
	if len(playerNames) == 0 {
		return nil, fmt.Errorf("the SlimServer is not connected to any players")
	}
	player, err := server.PlayerByName(playerNames[0])
	if err != nil {
		return nil, err
	}
	return player.(*Player), nil
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
	tracks, err := pl.Tracks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	player.TestPlaylistImplementation(t, pl.Playlist(), tracks[:3])
}
