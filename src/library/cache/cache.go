package cache

import (
	"fmt"
	"sync"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/util"
)

// A Cache wraps a Player and keeps a local copy of it's library.
//
// The copy is kept synchronized by listening for update events from the
// player.
type Cache struct {
	player.Player
	util.Emitter

	lock   sync.RWMutex
	tracks []library.Track
	index  map[string]*library.Track
	err    error
}

// NewCache wraps the specified player and caches it's library.
func NewCache(pl player.Player) *Cache {
	cache := &Cache{Player: pl}
	go cache.run()
	return cache
}

// Tracks implements the player.Library interface.
func (cache *Cache) Tracks() ([]library.Track, error) {
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

// TrackInfo implements the player.Library interface.
func (cache *Cache) TrackInfo(uris ...string) ([]library.Track, error) {
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

	results := make([]library.Track, len(uris))
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

// Events implements the player.Player interface.
func (cache *Cache) Events() *util.Emitter {
	return &cache.Emitter
}

func (cache *Cache) run() {
	listener := cache.Player.Events().Listen()
	defer cache.Player.Events().Unlisten(listener)

	// Reload tracks on startup.
	cache.lock.Lock()
	cache.reloadTracks()
	cache.lock.Unlock()
	cache.Emit(library.UpdateEvent{})

	for event := range listener {
		if _, ok := event.(library.UpdateEvent); ok {
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

	cache.tracks, cache.index = tracks, map[string]*library.Track{}
	for i, track := range cache.tracks {
		cache.index[track.URI] = &cache.tracks[i]
	}
}

func (cache *Cache) String() string {
	return fmt.Sprintf("Cache{%v}", cache.Player)
}
