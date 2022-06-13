package mpd

import (
	"context"
	"fmt"

	"github.com/fhs/gompd/v2/mpd"

	"trollibox/src/library"
)

type userPlaylist struct {
	player *Player
	name   string
}

func (plist userPlaylist) Insert(ctx context.Context, pos int, tracks ...library.Track) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Move(ctx context.Context, fromPos, toPos int) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Remove(ctx context.Context, positions ...int) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (plist userPlaylist) Tracks(ctx context.Context) ([]library.Track, error) {
	var tracks []library.Track
	err := plist.player.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistContents(plist.name)
		if err != nil {
			return err
		}
		tracks = make([]library.Track, len(songs))
		for i, song := range songs {
			if err := trackFromMpdSong(mpdc, song, &tracks[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return tracks, err
}

func (plist userPlaylist) Len(ctx context.Context) (int, error) {
	var length int
	err := plist.player.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		info, err := mpdc.PlaylistInfo(-1, -1)
		length = len(info)
		return err
	})
	if err != nil {
		return -1, err
	}
	return length, nil
}
