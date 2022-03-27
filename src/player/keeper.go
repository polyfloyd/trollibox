package player

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"trollibox/src/library"
)

// TrackMeta contains metadata for a track in a playlist.
type TrackMeta struct {
	// QueuedBy indicates by what entity a track was added.
	// Can be either "user" or "system".
	QueuedBy string
}

// The PlaylistMetaKeeper wraps a Playlist which does not track the meta
// information stored in a PlaylistTrack. This wrapper should be returned by
// players if these do not implement a proper system to store info specific to
// tracks in a playlist.
//
// Any operation performed on this playlist is propagated to the wrapped
// playlist and are safe for concurrent use.
type PlaylistMetaKeeper struct {
	Playlist[library.Track]

	tracks   []library.Track
	meta     []TrackMeta
	metaLock sync.Mutex
}

var _ Playlist[MetaTrack] = &PlaylistMetaKeeper{} // Enforce interface implementation.

func (kpr *PlaylistMetaKeeper) update(ctx context.Context) error {
	tracks, err := kpr.Playlist.Tracks(ctx)
	if err != nil {
		return err
	}

	inferDefault := func(target, source *TrackMeta) {
		if target.QueuedBy == "" {
			if source != nil && source.QueuedBy != "" {
				target.QueuedBy = source.QueuedBy
			} else {
				target.QueuedBy = "user"
			}
		}
	}

	newPlist := make([]library.Track, len(tracks))
	newMeta := make([]TrackMeta, len(tracks))
	found := map[string]int{}
outer:
	for i, track := range tracks {
		needIndex := found[track.URI] + 1
		duplicateIndex := 0
		for j, keptTrack := range kpr.tracks {
			if keptTrack.URI == track.URI {
				if duplicateIndex++; duplicateIndex == needIndex {
					newPlist[i] = keptTrack
					found[track.URI] = needIndex
					inferDefault(&newMeta[i], &kpr.meta[j])
					continue outer
				}
			}
		}

		newPlist[i] = track
		inferDefault(&newMeta[i], nil)
	}
	kpr.meta = newMeta
	kpr.tracks = newPlist
	return nil
}

// Insert implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Insert(ctx context.Context, pos int, metaTracks ...MetaTrack) error {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if kpr.meta == nil {
		if err := kpr.update(ctx); err != nil {
			return err
		}
	}

	tracks := make([]library.Track, len(metaTracks))
	meta := make([]TrackMeta, len(metaTracks))
	for i, mt := range metaTracks {
		tracks[i] = mt.Track
		meta[i] = mt.TrackMeta
	}
	if err := kpr.Playlist.Insert(ctx, pos, tracks...); err != nil {
		return err
	}

	if pos == -1 {
		kpr.tracks = append(kpr.tracks, tracks...)
		kpr.meta = append(kpr.meta, meta...)
	} else {
		kpr.tracks = append(kpr.tracks[:pos], append(tracks, kpr.tracks[pos:]...)...)
		kpr.meta = append(kpr.meta[:pos], append(meta, kpr.meta[pos:]...)...)
	}
	return nil
}

// Move implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Move(ctx context.Context, fromPos, toPos int) error {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if kpr.meta == nil {
		if err := kpr.update(ctx); err != nil {
			return err
		}
	}
	if err := kpr.Playlist.Move(ctx, fromPos, toPos); err != nil {
		return err
	}

	if fromPos >= len(kpr.meta) || toPos >= len(kpr.meta) {
		return fmt.Errorf("move positions out of range: (%v -> %v) len=%v", fromPos, toPos, len(kpr.meta))
	}
	delta := 0
	if fromPos > toPos {
		delta = 1
	}
	track := kpr.tracks[fromPos]
	kpr.tracks = append(kpr.tracks[:fromPos], kpr.tracks[fromPos+1:]...)
	kpr.tracks = append(kpr.tracks[:toPos+delta], append([]library.Track{track}, kpr.tracks[toPos+delta:]...)...)
	meta := kpr.meta[fromPos]
	kpr.meta = append(kpr.meta[:fromPos], kpr.meta[fromPos+1:]...)
	kpr.meta = append(kpr.meta[:toPos+delta], append([]TrackMeta{meta}, kpr.meta[toPos+delta:]...)...)
	return nil
}

// Remove implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Remove(ctx context.Context, positions ...int) error {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if kpr.meta == nil {
		if err := kpr.update(ctx); err != nil {
			return err
		}
	}
	sort.Ints(positions)
	if err := kpr.Playlist.Remove(ctx, positions...); err != nil {
		return err
	}

	for i := len(positions) - 1; i >= 0; i-- {
		pos := positions[i]
		if pos >= len(kpr.meta) {
			continue
		}
		kpr.tracks = append(kpr.tracks[:pos], kpr.tracks[pos+1:]...)
		kpr.meta = append(kpr.meta[:pos], kpr.meta[pos+1:]...)
	}
	return nil
}

// Tracks implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Tracks(ctx context.Context) ([]MetaTrack, error) {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if err := kpr.update(ctx); err != nil {
		return nil, err
	}

	metaTracks := make([]MetaTrack, len(kpr.tracks))
	for i, track := range kpr.tracks {
		metaTracks[i] = MetaTrack{Track: track, TrackMeta: kpr.meta[i]}
	}
	return metaTracks, nil
}
