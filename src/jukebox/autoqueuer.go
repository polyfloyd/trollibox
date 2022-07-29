package jukebox

import (
	"context"
	"math/rand"
	"time"
	"trollibox/src/filter"
	"trollibox/src/library"
	"trollibox/src/player"
)

// A TrackIterator is a type that produces a finite or infinite stream of tracks.
//
// Used by AutoAppend.
type TrackIterator interface {
	// Returns the next track from the iterator. If the bool flag is false, the
	// iterator has reached the end. The player that is requesting the next
	// track is specified.
	NextTrack(ctx context.Context, lib library.Library) (player.MetaTrack, bool)
}

type autoQueuer struct {
	player     player.Player
	filter     filter.Filter
	filterName string
	rand       *rand.Rand

	cancel chan struct{}
	err    chan error
}

func (aq *autoQueuer) stop() {
	close(aq.cancel)
}

// AutoAppend attaches a listener to the specified player. The iterator is used
// to get tracks which are played when the playlist of the player runs out.
//
// Sending a value over the returned channel interrupts the operation.
// Receiving from the channel blocks until no more tracks are available from
// the iterator or an error is encountered.
func autoQueue(pl player.Player, filterdb *filter.DB, filterName string) (*autoQueuer, error) {
	ft, err := filterdb.Get(filterName)
	if err != nil {
		return nil, err
	}

	aq := &autoQueuer{
		player:     pl,
		filter:     ft,
		filterName: filterName,
		rand:       rand.New(rand.NewSource(time.Now().Unix())),
		cancel:     make(chan struct{}),
		err:        make(chan error, 1),
	}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer close(aq.err)
		playerEvents := pl.Events().Listen(ctx)
		filterDBEvents := filterdb.Events().Listen(ctx)
	outer:
		for {
			select {
			case event := <-filterDBEvents:
				if uev, ok := event.(filter.UpdateEvent); ok && uev.Name == filterName {
					aq.filter = uev.Filter
				}
			case event := <-playerEvents:
				_, okA := event.(player.PlayStateEvent)
				_, okB := event.(player.PlaylistEvent)
				if !okA && !okB {
					continue
				}

				plist := pl.Playlist()
				status, err := pl.Status(ctx)
				if err != nil {
					aq.err <- err
					return
				}
				if status.PlayState != player.PlayStateStopped && status.TrackIndex != -1 {
					continue
				}

				metaTrack, ok := aq.nextTrack(ctx)
				if !ok {
					break outer
				}
				if err := plist.Insert(ctx, -1, metaTrack); err != nil {
					aq.err <- err
					return
				}
				plistLen, err := plist.Len(ctx)
				if err != nil {
					aq.err <- err
					return
				}
				if err := pl.SetState(ctx, player.PlayStatePlaying); err != nil {
					aq.err <- err
					return
				}
				if err := pl.SetTrackIndex(ctx, plistLen-1); err != nil {
					aq.err <- err
					return
				}

			case <-aq.cancel:
				break outer
			}
		}
	}()
	return aq, nil
}

func (aq *autoQueuer) nextTrack(ctx context.Context) (player.MetaTrack, bool) {
	tracks, err := aq.player.Library().Tracks(ctx)
	if err != nil {
		return player.MetaTrack{}, false
	}

	results, _ := filter.Tracks(ctx, aq.filter, tracks)
	if len(results) == 0 {
		return player.MetaTrack{}, false
	}
	return player.MetaTrack{
		Track:    results[aq.rand.Intn(len(results))].Track,
		QueuedBy: "system",
	}, true
}
