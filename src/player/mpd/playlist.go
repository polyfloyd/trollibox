package mpd

import (
	"fmt"

	player "../"
	"github.com/polyfloyd/gompd/mpd"
)

type userPlaylist struct {
	player *Player
	name   string
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
	var tracks []player.Track
	err := plist.player.withMpd(func(mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistContents(plist.name)
		if err != nil {
			return err
		}
		tracks = make([]player.Track, len(songs))
		for i, song := range songs {
			if err := trackFromMpdSong(mpdc, &song, &tracks[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return tracks, err
}

func (plist userPlaylist) Len() (int, error) {
	var length int
	err := plist.player.withMpd(func(mpdc *mpd.Client) error {
		info, err := mpdc.PlaylistInfo(-1, -1)
		length = len(info)
		return err
	})
	if err != nil {
		return -1, err
	}
	return length, nil
}
