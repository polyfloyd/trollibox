package filter

import (
	"context"
	"math/rand"
	"time"

	"trollibox/src/library"
	"trollibox/src/player"
)

type randFilterIterator struct {
	filter Filter
	rand   *rand.Rand
}

// RandomIterator creates a track iterator which will use the supplied filter
// to pick random tracks.
func RandomIterator(filter Filter) player.TrackIterator {
	return &randFilterIterator{
		filter: filter,
		rand:   rand.New(rand.NewSource(time.Now().Unix())),
	}
}

func (it randFilterIterator) NextTrack(ctx context.Context, lib library.Library) (player.MetaTrack, bool) {
	tracks, err := lib.Tracks(ctx)
	if err != nil {
		return player.MetaTrack{}, false
	}

	results, _ := Tracks(ctx, it.filter, tracks)
	if len(results) == 0 {
		return player.MetaTrack{}, false
	}
	return player.MetaTrack{
		Track:    results[it.rand.Intn(len(results))].Track,
		QueuedBy: "system",
	}, true
}
