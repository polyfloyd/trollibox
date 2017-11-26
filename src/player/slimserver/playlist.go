package slimserver

import (
	"fmt"
	"strconv"

	"github.com/polyfloyd/trollibox/src/player"
)

type userPlaylist struct {
	player *Player
	id     string
}

func (plist userPlaylist) Insert(pos int, tracks ...player.Track) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Move(fromPos, toPos int) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Remove(positions ...int) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Tracks() ([]player.Track, error) {
	numTracks, err := plist.Len()
	if err != nil {
		return nil, err
	}
	return plist.player.Serv.decodeTracks("id", numTracks, "playlists", "tracks", "0", strconv.Itoa(numTracks), "playlist_id:"+plist.id, "tags:"+trackTags)
}

func (plist userPlaylist) Len() (int, error) {
	attrs, err := plist.player.Serv.requestAttrs("playlists", "tracks", "0", "0", "playlist_id:"+plist.id)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(attrs["count"])
}
