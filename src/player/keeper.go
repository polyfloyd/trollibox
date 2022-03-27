package player

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"trollibox/src/library"
)

// The PlaylistMetaKeeper wraps a Playlist which does not track the meta
// information stored in a PlaylistTrack. This wrapper should be returned by
// players if these do not implement a proper system to store info specific to
// tracks in a playlist.
//
// Any operation performed on this playlist is propagated to the wrapped
// playlist and are safe for concurrent use.
type PlaylistMetaKeeper struct {
	Playlist[library.Track]

	tracks     []MetaTrack
	tracksLock sync.Mutex
}

var _ Playlist[MetaTrack] = &PlaylistMetaKeeper{} // Enforce interface implementation.

func (kpr *PlaylistMetaKeeper) update(ctx context.Context) error {
	tracks, err := kpr.Playlist.Tracks(ctx)
	if err != nil {
		return err
	}

	inferDefault := func(target, source *MetaTrack) {
		if target.QueuedBy == "" {
			if source != nil && source.QueuedBy != "" {
				target.QueuedBy = source.QueuedBy
			} else {
				target.QueuedBy = "user"
			}
		}
	}

	newPlist := make([]MetaTrack, len(tracks))
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
					inferDefault(&newPlist[i], &kpr.tracks[j])
					continue outer
				}
			}
		}

		newPlist[i] = MetaTrack{Track: track}
		inferDefault(&newPlist[i], nil)
	}
	kpr.tracks = newPlist
	return nil
}

// Insert implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Insert(ctx context.Context, pos int, metaTracks ...MetaTrack) error {
	kpr.tracksLock.Lock()
	defer kpr.tracksLock.Unlock()
	if kpr.tracks == nil {
		if err := kpr.update(ctx); err != nil {
			return err
		}
	}

	tracks := make([]library.Track, len(metaTracks))
	for i, mt := range metaTracks {
		tracks[i] = mt.Track
	}
	if err := kpr.Playlist.Insert(ctx, pos, tracks...); err != nil {
		return err
	}

	if pos == -1 {
		kpr.tracks = append(kpr.tracks, metaTracks...)
	} else {
		kpr.tracks = append(kpr.tracks[:pos], append(metaTracks, kpr.tracks[pos:]...)...)
	}
	return nil
}

// Move implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Move(ctx context.Context, fromPos, toPos int) error {
	kpr.tracksLock.Lock()
	defer kpr.tracksLock.Unlock()
	if kpr.tracks == nil {
		if err := kpr.update(ctx); err != nil {
			return err
		}
	}

	if err := kpr.Playlist.Move(ctx, fromPos, toPos); err != nil {
		return err
	}

	if fromPos >= len(kpr.tracks) || toPos >= len(kpr.tracks) {
		return fmt.Errorf("move positions out of range: (%v -> %v) len=%v", fromPos, toPos, len(kpr.tracks))
	}
	delta := 0
	if fromPos > toPos {
		delta = 1
	}
	track := kpr.tracks[fromPos]
	kpr.tracks = append(kpr.tracks[:fromPos], kpr.tracks[fromPos+1:]...)
	kpr.tracks = append(kpr.tracks[:toPos+delta], append([]MetaTrack{track}, kpr.tracks[toPos+delta:]...)...)
	return nil
}

// Remove implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Remove(ctx context.Context, positions ...int) error {
	kpr.tracksLock.Lock()
	defer kpr.tracksLock.Unlock()
	if kpr.tracks == nil {
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
		if pos >= len(kpr.tracks) {
			continue
		}
		kpr.tracks = append(kpr.tracks[:pos], kpr.tracks[pos+1:]...)
	}
	return nil
}

// Tracks implements the player.Playlist interface.
func (kpr *PlaylistMetaKeeper) Tracks(ctx context.Context) ([]MetaTrack, error) {
	kpr.tracksLock.Lock()
	defer kpr.tracksLock.Unlock()
	if err := kpr.update(ctx); err != nil {
		return nil, err
	}

	return kpr.tracks, nil
}
