package jukebox

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"trollibox/src/filter"
	"trollibox/src/filter/keyed"
	"trollibox/src/library"
	"trollibox/src/library/stream"
	"trollibox/src/player"
	"trollibox/src/util"
)

// ErrPlayerUnavailable is returned from functions that operate on player state
// when a player is registered but unreachable for any reason.
var ErrPlayerUnavailable = fmt.Errorf("the player is not available")

// Jukebox augments one or more players with with filters, streams and other
// functionality.
type Jukebox struct {
	players       player.List
	filterdb      *filter.DB
	streamdb      *stream.DB
	defaultPlayer string
}

func NewJukebox(players player.List, filterdb *filter.DB, streamdb *stream.DB, defaultPlayer string) *Jukebox {
	return &Jukebox{
		players:       players,
		filterdb:      filterdb,
		streamdb:      streamdb,
		defaultPlayer: defaultPlayer,
	}
}

func (jb *Jukebox) Players(ctx context.Context) ([]string, error) {
	return jb.players.PlayerNames()
}

func (jb *Jukebox) DefaultPlayer(ctx context.Context) (string, error) {
	if jb.defaultPlayer != "" {
		if pl, err := jb.players.PlayerByName(jb.defaultPlayer); err == nil && pl != nil {
			return jb.defaultPlayer, nil
		}
	}

	names, err := jb.players.PlayerNames()
	if err != nil {
		return "", fmt.Errorf("could not auto select default player: %v", err)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("could not auto select default player: no players present")
	}
	return names[0], nil
}

func (jb *Jukebox) PlayerTrackIndex(ctx context.Context, playerName string) (int, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return -1, err
	}
	return pl.TrackIndex()
}

func (jb *Jukebox) SetPlayerTrackIndex(ctx context.Context, playerName string, index int, relative bool) error {
	pl, err := jb.player(playerName)
	if err != nil {
		return err
	}
	if relative {
		cur, err := pl.TrackIndex()
		if err != nil {
			return err
		}
		index += cur
	}
	return pl.SetTrackIndex(index)
}

func (jb *Jukebox) PlayerTime(ctx context.Context, playerName string) (time.Duration, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return 0, err
	}
	return pl.Time()
}

func (jb *Jukebox) SetPlayerTime(ctx context.Context, playerName string, t time.Duration) error {
	pl, err := jb.player(playerName)
	if err != nil {
		return err
	}
	return pl.SetTime(t)
}

func (jb *Jukebox) PlayerState(ctx context.Context, playerName string) (player.PlayState, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return player.PlayStateInvalid, err
	}
	return pl.State()
}

func (jb *Jukebox) SetPlayerState(ctx context.Context, playerName string, state player.PlayState) error {
	pl, err := jb.player(playerName)
	if err != nil {
		return err
	}
	return pl.SetState(state)
}

func (jb *Jukebox) PlayerVolume(ctx context.Context, playerName string) (int, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return 0, err
	}
	return pl.Volume()
}

func (jb *Jukebox) SetPlayerVolume(ctx context.Context, playerName string, vol int) error {
	pl, err := jb.player(playerName)
	if err != nil {
		return err
	}
	return pl.SetVolume(vol)
}

func (jb *Jukebox) Tracks(ctx context.Context, playerName string) ([]library.Track, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return nil, err
	}
	return pl.Library().Tracks()
}

func (jb *Jukebox) TrackArt(ctx context.Context, playerName, uri string) (io.Reader, string, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return nil, "", err
	}
	image, mime := pl.Library().TrackArt(uri)
	return image, mime, nil
}

func (jb *Jukebox) SearchTracks(ctx context.Context, playerName, query string, untagged []string) ([]filter.SearchResult, error) {
	compiledQuery, err := keyed.CompileQuery(query, untagged)
	if err != nil {
		return nil, err
	}
	pl, err := jb.player(playerName)
	if err != nil {
		return nil, err
	}
	tracks, err := pl.Library().Tracks()
	if err != nil {
		return nil, err
	}
	results := filter.Tracks(compiledQuery, tracks)
	sort.Sort(filter.ByNumMatches(results))
	return results, nil
}

func (jb *Jukebox) PlayerPlaylist(ctx context.Context, playerName string) (player.MetaPlaylist, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return nil, err
	}
	return pl.Playlist(), nil
}

func (jb *Jukebox) PlayerLibraries(ctx context.Context, playerName string) ([]library.Library, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return nil, err
	}
	return []library.Library{jb.streamdb, pl.Library()}, nil
}

func (jb *Jukebox) PlayerLibrary(ctx context.Context, playerName string) (library.Library, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return nil, err
	}
	return pl.Library(), nil
}

func (jb *Jukebox) PlayerEvents(ctx context.Context, playerName string) (*util.Emitter, error) {
	pl, err := jb.player(playerName)
	if err != nil {
		return nil, err
	}
	return pl.Events(), nil
}

func (jb *Jukebox) player(name string) (player.Player, error) {
	pl, err := jb.players.PlayerByName(name)
	if err != nil {
		return nil, err
	}
	if !pl.Available() {
		return nil, ErrPlayerUnavailable
	}
	return pl, nil
}

func (jb *Jukebox) FilterDB() *filter.DB {
	return jb.filterdb
}

func (jb *Jukebox) StreamDB() *stream.DB {
	return jb.streamdb
}
