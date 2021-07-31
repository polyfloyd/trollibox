package mpd

import (
	"fmt"

	"github.com/fhs/gompd/v2/mpd"

	"trollibox/src/library"
)

type userPlaylist struct {
	player *Player
	name   string
}

func (plist userPlaylist) Insert(pos int, tracks ...library.Track) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Move(fromPos, toPos int) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Remove(positions ...int) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Tracks() ([]library.Track, error) {
	var tracks []library.Track
	err := plist.player.withMpd(func(mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistContents(plist.name)
		if err != nil {
			return err
		}
		tracks = make([]library.Track, len(songs))
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
