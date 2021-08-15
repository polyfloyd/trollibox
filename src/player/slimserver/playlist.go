package slimserver

import (
	"context"
	"fmt"
	"strconv"

	"trollibox/src/library"
)

type userPlaylist struct {
	player *Player
	id     string
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
	numTracks, err := plist.Len(ctx)
	if err != nil {
		return nil, err
	}
	return plist.player.Serv.decodeTracks("id", numTracks, "playlists", "tracks", "0", strconv.Itoa(numTracks), "playlist_id:"+plist.id, "tags:"+trackTags)
}

func (plist userPlaylist) Len(ctx context.Context) (int, error) {
	attrs, err := plist.player.Serv.requestAttrs("playlists", "tracks", "0", "0", "playlist_id:"+plist.id)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(attrs["count"])
}
