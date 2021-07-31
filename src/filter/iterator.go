package filter

import (
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

func (it randFilterIterator) NextTrack(lib library.Library) (library.Track, player.TrackMeta, bool) {
	tracks, err := lib.Tracks()
	if err != nil {
		return library.Track{}, player.TrackMeta{}, false
	}

	results := Tracks(it.filter, tracks)
	if len(results) == 0 {
		return library.Track{}, player.TrackMeta{}, false
	}
	return results[it.rand.Intn(len(results))].Track, player.TrackMeta{QueuedBy: "system"}, true
}
