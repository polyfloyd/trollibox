package player

import (
	"sync"
)

type TrackCache struct {
	Player

	lock   sync.RWMutex
	tracks []Track
	index  map[string]*Track
	err    error
}

func (cache *TrackCache) TrackInfo(identites ...TrackIdentity) ([]Track, error) {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	if cache.err != nil {
		return nil, cache.err
	}
	if cache.tracks == nil {
		cache.lock.RUnlock()
		cache.lock.Lock()
		cache.reloadTrackInfo()
		cache.lock.Unlock()
		cache.lock.RLock()
	}

	if len(identites) == 0 {
		return cache.tracks, nil
	}

	results := make([]Track, len(identites))
	for i, id := range identites {
		if track, ok := cache.index[id.TrackUri()]; ok {
			results[i] = *track
		} else {
			tracks, err := cache.Player.TrackInfo(id)
			if err != nil {
				return nil, err
			}
			results[i] = tracks[0]
		}
	}
	return results, nil
}

func (cache *TrackCache) Run() {
	listener := cache.Events().Listen()
	defer cache.Events().Unlisten(listener)

	for event := range listener {
		if event != "tracks" {
			continue
		}
		cache.lock.Lock()
		cache.reloadTrackInfo()
		cache.lock.Unlock()
	}
}

func (cache *TrackCache) reloadTrackInfo() {
	tracks, err := cache.Player.TrackInfo()
	if err != nil {
		cache.err = err
		cache.tracks, cache.index = nil, nil
		return
	}

	cache.tracks, cache.index = tracks, map[string]*Track{}
	for i, track := range cache.tracks {
		cache.index[track.Uri] = &cache.tracks[i]
	}
}
