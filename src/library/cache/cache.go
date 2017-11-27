package cache

import (
	"fmt"
	"sync"

	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/util"
)

type Cache struct {
	player.Player
	util.Emitter

	lock   sync.RWMutex
	tracks []player.Track
	index  map[string]*player.Track
	err    error
}

func (cache *Cache) Tracks() ([]player.Track, error) {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	if cache.tracks == nil {
		cache.lock.RUnlock()
		cache.lock.Lock()
		if cache.tracks == nil {
			cache.reloadTracks()
		}
		cache.lock.Unlock()
		cache.lock.RLock()
	}
	return cache.tracks, cache.err
}

func (cache *Cache) TrackInfo(uris ...string) ([]player.Track, error) {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	if cache.tracks == nil {
		cache.lock.RUnlock()
		cache.lock.Lock()
		if cache.tracks == nil {
			cache.reloadTracks()
		}
		cache.lock.Unlock()
		cache.lock.RLock()
	}
	if cache.err != nil {
		return nil, cache.err
	}

	results := make([]player.Track, len(uris))
	for i, uri := range uris {
		if track, ok := cache.index[uri]; ok {
			results[i] = *track
		} else {
			tracks, err := cache.Player.TrackInfo(uri)
			if err != nil {
				return nil, err
			}
			results[i] = tracks[0]
		}
	}
	return results, nil
}

func (cache *Cache) Events() *util.Emitter {
	return &cache.Emitter
}

func (cache *Cache) Run() {
	listener := cache.Player.Events().Listen()
	defer cache.Player.Events().Unlisten(listener)

	// Reload tracks on startup.
	cache.lock.Lock()
	cache.reloadTracks()
	cache.lock.Unlock()
	cache.Emit("tracks")

	for event := range listener {
		if event == "tracks" {
			cache.lock.Lock()
			cache.reloadTracks()
			cache.lock.Unlock()
		}
		cache.Emit(event)
	}
}

func (cache *Cache) reloadTracks() {
	tracks, err := cache.Player.Tracks()
	if err != nil {
		cache.err = err
		cache.tracks, cache.index = nil, nil
		return
	}

	cache.tracks, cache.index = tracks, map[string]*player.Track{}
	for i, track := range cache.tracks {
		cache.index[track.URI] = &cache.tracks[i]
	}
}

func (cache *Cache) String() string {
	return fmt.Sprintf("Cache{%v}", cache.Player)
}
