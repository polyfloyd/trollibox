package jukebox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

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

type PlayerAutoQueuerEvent struct {
	PlayerName string
	FilterName string
}

// Jukebox augments one or more players with with filters, streams and other
// functionality.
type Jukebox struct {
	util.Emitter

	players       player.List
	filterdb      *filter.DB
	streamdb      *stream.DB
	defaultPlayer string

	autoQueuers         sync.Map // map[string]*autoQueuer
	autoQueuerStateFile string
}

func NewJukebox(players player.List, filterdb *filter.DB, streamdb *stream.DB, defaultPlayer, autoQueuerStateFile string) *Jukebox {
	jb := &Jukebox{
		players:             players,
		filterdb:            filterdb,
		streamdb:            streamdb,
		defaultPlayer:       defaultPlayer,
		autoQueuerStateFile: autoQueuerStateFile,
	}

	if b, err := os.ReadFile(autoQueuerStateFile); err == nil {
		var autoQueuerState map[string]string
		if err := yaml.Unmarshal(b, &autoQueuerState); err == nil {
			for player, filter := range autoQueuerState {
				_ = jb.SetPlayerAutoQueuerFilter(context.Background(), player, filter)
			}
		}
	}

	return jb
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

func (jb *Jukebox) PlayerAutoQueuerFilters(ctx context.Context) map[string]string {
	playerFilters := map[string]string{}
	jb.autoQueuers.Range(func(k, v interface{}) bool {
		playerFilters[k.(string)] = v.(*autoQueuer).filterName
		return true
	})
	return playerFilters
}

func (jb *Jukebox) SetPlayerAutoQueuerFilter(ctx context.Context, playerName, filterName string) error {
	pl, err := jb.players.PlayerByName(playerName)
	if err != nil {
		return err
	}

	if aq, ok := jb.autoQueuers.LoadAndDelete(playerName); ok {
		aq.(*autoQueuer).stop()
		log.WithField("player", playerName).Debugf("Stopped existing auto queuer")
	}

	if filterName == "" {
		jb.saveAutoQueuerState()
		jb.Emit(PlayerAutoQueuerEvent{PlayerName: playerName, FilterName: ""})
		return nil
	}

	aq, err := autoQueue(pl, jb.filterdb, filterName)
	if err != nil {
		return err
	}
	go func() {
		if err := <-aq.err; err != nil {
			log.WithField("player", playerName).Errorf("Auto queuer: %v", err)
		}
	}()
	jb.autoQueuers.Store(playerName, aq)
	jb.saveAutoQueuerState()
	jb.Emit(PlayerAutoQueuerEvent{PlayerName: playerName, FilterName: filterName})
	log.WithField("player", playerName).Debugf("Set auto queuer to %q", filterName)

	return nil
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

func (jb *Jukebox) saveAutoQueuerState() {
	state := jb.PlayerAutoQueuerFilters(context.Background())
	b, err := yaml.Marshal(state)
	if err != nil {
		log.Warnf("Unable to marshal auto queuer state")
		return
	}
	if err := os.WriteFile(jb.autoQueuerStateFile, b, 0o644); err != nil {
		log.WithField("file", jb.autoQueuerStateFile).Warnf("Unable to save auto queuer state file")
	}
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
