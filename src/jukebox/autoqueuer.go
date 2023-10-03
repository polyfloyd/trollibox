package jukebox

import (
	"context"
	"math/rand"

	"trollibox/src/filter"
	"trollibox/src/library"
	"trollibox/src/player"
)

type autoQueuerQueue struct {
	tracks []library.Track
	index  int
}

func newQueue(ctx context.Context, pl player.Player, ft filter.Filter) (*autoQueuerQueue, error) {
	tracks, err := pl.Library().Tracks(ctx)
	if err != nil {
		return nil, err
	}
	results, err := filter.Tracks(ctx, ft, tracks)
	if err != nil {
		return nil, err
	}

	shuffledTracks := make([]library.Track, len(results))
	for i, t := range results {
		shuffledTracks[i] = t.Track
	}
	rand.Shuffle(len(shuffledTracks), func(i, j int) {
		shuffledTracks[i], shuffledTracks[j] = shuffledTracks[j], shuffledTracks[i]
	})

	return &autoQueuerQueue{tracks: shuffledTracks, index: 0}, nil
}

func (q *autoQueuerQueue) nextTrack() (player.MetaTrack, bool) {
	if len(q.tracks) == 0 {
		return player.MetaTrack{}, false
	}
	track := q.tracks[q.index]
	q.index = (q.index + 1) % len(q.tracks)
	return player.MetaTrack{Track: track, QueuedBy: "system"}, true
}

type autoQueuer struct {
	queue      *autoQueuerQueue
	filterName string

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

	queue, err := newQueue(context.Background(), pl, ft)
	if err != nil {
		return nil, err
	}

	aq := &autoQueuer{
		filterName: filterName,
		queue:      queue,
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
					queue, err := newQueue(ctx, pl, uev.Filter)
					if err != nil {
						aq.err <- err
						return
					}
					aq.queue = queue
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

				metaTrack, ok := aq.queue.nextTrack()
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
