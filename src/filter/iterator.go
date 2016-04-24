package filter

import (
	"math/rand"
	"time"

	"../player"
)

type randFilterIterator struct {
	filter Filter
	rand   *rand.Rand
}

// Creates a track iterator which will use the supplied filter to pick random
// tracks.
func RandomIterator(filter Filter) player.TrackIterator {
	return &randFilterIterator{
		filter: filter,
		rand:   rand.New(rand.NewSource(time.Now().Unix())),
	}
}

func (it randFilterIterator) NextTrack(pl player.Player) (player.Track, player.TrackMeta, bool) {
	tracks, err := pl.Tracks()
	if err != nil {
		return player.Track{}, player.TrackMeta{}, false
	}

	results := FilterTracks(it.filter, tracks)
	if len(results) == 0 {
		return player.Track{}, player.TrackMeta{}, false
	}
	return results[it.rand.Intn(len(results))].Track, player.TrackMeta{QueuedBy: "system"}, true
}
