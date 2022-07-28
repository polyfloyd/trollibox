package jukebox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"trollibox/src/filter"
	"trollibox/src/filter/keyed"
	"trollibox/src/library"
	"trollibox/src/library/stream"
	"trollibox/src/player"
	"trollibox/src/util"
)

var (
	// ErrPlayerUnavailable is returned from functions that operate on player state
	// when a player is registered but unreachable for any reason.
	ErrPlayerUnavailable = player.ErrUnavailable

	ErrPlayerNotFound = player.ErrPlayerNotFound
)

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
		if _, err := jb.players.PlayerByName(jb.defaultPlayer); errors.Is(err, player.ErrPlayerNotFound) {
			// Fallthrough.
		} else if err != nil {
			return "", err
		} else {
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

func (jb *Jukebox) PlayerStatus(ctx context.Context, playerName string) (*player.Status, error) {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return nil, err
	}
	return pl.Status(ctx)
}

func (jb *Jukebox) SetPlayerTrackIndex(ctx context.Context, playerName string, index int, relative bool) error {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return err
	}
	if relative {
		status, err := pl.Status(ctx)
		if err != nil {
			return err
		}
		index += status.TrackIndex
	}
	return pl.SetTrackIndex(ctx, index)
}

func (jb *Jukebox) SetPlayerTime(ctx context.Context, playerName string, t time.Duration) error {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return err
	}
	return pl.SetTime(ctx, t)
}

func (jb *Jukebox) SetPlayerState(ctx context.Context, playerName string, state player.PlayState) error {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return err
	}
	return pl.SetState(ctx, state)
}

func (jb *Jukebox) SetPlayerVolume(ctx context.Context, playerName string, vol int) error {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return err
	}
	return pl.SetVolume(ctx, vol)
}

func (jb *Jukebox) Tracks(ctx context.Context, playerName string) ([]library.Track, error) {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return nil, err
	}
	return pl.Library().Tracks(ctx)
}

func (jb *Jukebox) TrackArt(ctx context.Context, playerName, uri string) (*library.Art, error) {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return nil, err
	}
	libs := []library.Library{jb.streamdb, pl.Library()}

	var art *library.Art
	for _, lib := range libs {
		art, err = lib.TrackArt(ctx, uri)
		if err == nil {
			break
		}
	}
	return art, err
}

func (jb *Jukebox) SearchTracks(ctx context.Context, playerName, query string, untagged []string) ([]filter.SearchResult, error) {
	compiledQuery, err := keyed.CompileQuery(query, untagged)
	if err != nil {
		return nil, err
	}
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return nil, err
	}
	tracks, err := pl.Library().Tracks(ctx)
	if err != nil {
		return nil, err
	}
	results, err := filter.Tracks(ctx, compiledQuery, tracks)
	if err != nil {
		return nil, err
	}
	sort.Sort(filter.ByNumMatches(results))
	return results, nil
}

func (jb *Jukebox) PlayerPlaylist(ctx context.Context, playerName string) (player.Playlist[player.MetaTrack], error) {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return nil, err
	}
	libs := []library.Library{pl.Library(), jb.streamdb}
	return playerPlaylist{libraries: libs, Playlist: pl.Playlist()}, nil
}

func (jb *Jukebox) PlayerPlaylistInsertAt(ctx context.Context, playerName, at string, pos int, tracks []player.MetaTrack) error {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return err
	}

	if at == "Next" {
		status, err := pl.Status(ctx)
		if err != nil {
			return err
		}
		pos = status.TrackIndex + 1
	} else if at == "End" {
		pos = -1
	}

	return pl.Playlist().Insert(ctx, pos, tracks...)
}

func (jb *Jukebox) PlayerEvents(ctx context.Context, playerName string) (*util.Emitter, error) {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return nil, err
	}
	return pl.Events(), nil
}

func (jb *Jukebox) FilterDB() *filter.DB {
	return jb.filterdb
}

func (jb *Jukebox) StreamDB() *stream.DB {
	return jb.streamdb
}

type playerPlaylist struct {
	libraries []library.Library
	player.Playlist[player.MetaTrack]
}

func (pl playerPlaylist) Tracks(ctx context.Context) ([]player.MetaTrack, error) {
	tracks, err := pl.Playlist.Tracks(ctx)
	if err != nil {
		return nil, err
	}
	uris := make([]string, len(tracks))
	for i, t := range tracks {
		uris[i] = t.URI
	}

	freshTracks, err := library.AllTrackInfo(ctx, pl.libraries, uris...)
	if err != nil {
		return nil, err
	}
	for i, ft := range freshTracks {
		tracks[i].Track = ft
	}
	return tracks, nil
}
