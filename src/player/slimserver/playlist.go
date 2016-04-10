package slimserver

import (
	"fmt"
	"strconv"

	player "../"
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
	tracks := make([]player.Track, numTracks)
	for i := 0; i < numTracks; i++ {
		attrs, err := plist.player.Serv.requestAttrs("playlists", "tracks", strconv.Itoa(i), "1", "playlist_id:"+plist.id, "tags:"+trackTags)
		if err != nil {
			return nil, err
		}
		for key, value := range attrs {
			setSlimAttr(plist.player.Serv, &tracks[i], key, value)
		}
	}
	return tracks, nil
}

func (plist userPlaylist) Len() (int, error) {
	attrs, err := plist.player.Serv.requestAttrs("playlists", "tracks", "0", "0", "playlist_id:"+plist.id)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(attrs["count"])
}
