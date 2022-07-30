package cache

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"trollibox/src/library"
	"trollibox/src/util"
)

// A Cache wraps a Library and keeps a local copy of it's library.
//
// The copy is kept synchronized by listening for update events from the
// library.
type Cache struct {
	library.Library
	util.Emitter

	lock   sync.RWMutex
	tracks []library.Track
	index  map[string]*library.Track
	err    error
}

// NewCache wraps the specified library and caches it's contents.
func NewCache(lib library.Library) *Cache {
	cache := &Cache{Library: lib}
	go cache.run()
	return cache
}

// Tracks implements the library.Library interface.
func (cache *Cache) Tracks(ctx context.Context) ([]library.Track, error) {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	if cache.tracks == nil {
		cache.lock.RUnlock()
		cache.lock.Lock()
		if cache.tracks == nil {
			cache.reloadTracks(ctx)
		}
		cache.lock.Unlock()
		cache.lock.RLock()
	}
	return cache.tracks, cache.err
}

// TrackInfo implements the library.Library interface.
func (cache *Cache) TrackInfo(ctx context.Context, uris ...string) ([]library.Track, error) {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	if cache.tracks == nil {
		cache.lock.RUnlock()
		cache.lock.Lock()
		if cache.tracks == nil {
			cache.reloadTracks(ctx)
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
			tracks, err := cache.Library.TrackInfo(ctx, uri)
			if err != nil {
				return nil, err
			}
			results[i] = tracks[0]
		}
	}
	return results, nil
}

// Events implements the util.Eventer interface.
func (cache *Cache) Events() *util.Emitter {
	return &cache.Emitter
}

func (cache *Cache) run() {
	ctx := context.Background()
	listener := cache.Library.Events().Listen(ctx)

	// Reload tracks on startup.
	cache.lock.Lock()
	cache.reloadTracks(context.Background())
	cache.lock.Unlock()
	cache.Emit(library.UpdateEvent{})

	for event := range listener {
		if _, ok := event.(library.UpdateEvent); ok {
			cache.lock.Lock()
			cache.reloadTracks(context.Background())
			cache.lock.Unlock()
		}
		cache.Emit(event)
	}
}

func (cache *Cache) reloadTracks(ctx context.Context) {
	log.Infof("%v: Reloading tracks", cache)

	tracks, err := cache.Library.Tracks(ctx)
	if err != nil {
		cache.err = err
		cache.tracks, cache.index = nil, nil
		return
	}

	cache.tracks, cache.index = tracks, map[string]*library.Track{}
	for i, track := range cache.tracks {
		cache.index[track.URI] = &cache.tracks[i]
	}

	log.Infof("%v: Done reloading tracks", cache)
}

func (cache *Cache) String() string {
	return fmt.Sprintf("Cache{%v}", cache.Library)
}
