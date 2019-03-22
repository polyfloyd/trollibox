package jukebox

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/polyfloyd/trollibox/src/filter"
	"github.com/polyfloyd/trollibox/src/filter/keyed"
	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/library/netmedia"
	"github.com/polyfloyd/trollibox/src/library/raw"
	"github.com/polyfloyd/trollibox/src/library/stream"
	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/util"
)

var ErrPlayerUnavailable = fmt.Errorf("the player is not available")

type Jukebox struct {
	players   player.List
	netServer *netmedia.Server
	filterdb  *filter.DB
	streamdb  *stream.DB
	rawServer *raw.Server
}

func NewJukebox(players player.List, netServer *netmedia.Server, filterdb *filter.DB, streamdb *stream.DB, rawServer *raw.Server) *Jukebox {
	return &Jukebox{
		players:   players,
		netServer: netServer,
		filterdb:  filterdb,
		streamdb:  streamdb,
		rawServer: rawServer,
	}
}

func (jb *Jukebox) Players(ctx context.Context) ([]string, error) {
	return jb.players.PlayerNames()
}

func (jb *Jukebox) AppendRawFile(ctx context.Context, playerName string, file io.Reader, filename string) error {
	pl, err := jb.player(playerName)
	if err != nil {
		return err
	}

	track, errs := jb.rawServer.Add(ctx, filename, nil, "", func(ctx context.Context, w io.Writer) error {
		_, err := io.Copy(w, file)
		return err
	})
	if err := <-errs; err != nil {
		return err
	}

	// Launch a goroutine that will check whether the track is still in
	// the player's playlist. If it is not, the track is removed from
	// the server.
	go jb.removeRawTrack(playerName, track, jb.rawServer)

	return pl.Playlist().InsertWithMeta(-1, []library.Track{track}, []player.TrackMeta{
		{QueuedBy: "user"},
	})
}

func (jb *Jukebox) AppendNetFile(ctx context.Context, playerName, url string) error {
	pl, err := jb.player(playerName)
	if err != nil {
		return err
	}

	track, errc := jb.netServer.Download(url)
	go func() {
		if err := <-errc; err != nil {
			log.Error(err)
		}
	}()

	// Launch a goroutine that will check whether the track is still in
	// the player's playlist. If it is not, the track is removed from
	// the server.
	go jb.removeRawTrack(playerName, track, jb.netServer.RawServer())

	return pl.Playlist().InsertWithMeta(-1, []library.Track{track}, []player.TrackMeta{
		{QueuedBy: "user"},
	})
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
	return []library.Library{
		jb.streamdb,
		jb.rawServer,
		pl.Library(),
	}, nil
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

func (jb *Jukebox) removeRawTrack(playerName string, track library.Track, rawServer *raw.Server) {
	emitter, err := jb.PlayerEvents(context.Background(), playerName)
	if err != nil {
		log.Error(err)
		return
	}
	events := emitter.Listen()
	defer emitter.Unlisten(events)
outer:
	for event := range events {
		if _, ok := event.(player.PlaylistEvent); !ok {
			continue
		}
		plist, err := jb.PlayerPlaylist(context.Background(), playerName)
		if err != nil {
			log.Error(err)
			break
		}
		tracks, err := plist.Tracks()
		if err != nil {
			log.Error(err)
			break
		}
		for _, plTrack := range tracks {
			if track.URI == plTrack.URI {
				continue outer
			}
		}
		break
	}
	rawServer.Remove(track.URI)
}

func (jb *Jukebox) FilterDB() *filter.DB {
	return jb.filterdb
}

func (jb *Jukebox) StreamDB() *stream.DB {
	return jb.streamdb
}

func (jb *Jukebox) RawServer() *raw.Server {
	return jb.rawServer
}
