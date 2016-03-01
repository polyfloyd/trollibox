package player

import (
	"fmt"
	"sort"
	"sync"
)

type TrackMeta struct {
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
	Playlist

	tracks   []Track
	meta     []TrackMeta
	metaLock sync.Mutex
}

func (kpr *PlaylistMetaKeeper) update() error {
	tracks, err := kpr.Playlist.Tracks()
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

	newPlist := make([]Track, len(tracks))
	newMeta := make([]TrackMeta, len(tracks))
	found := map[string]int{}
outer:
	for i, track := range tracks {
		needIndex := found[track.Uri] + 1
		duplicateIndex := 0
		for j, keptTrack := range kpr.tracks {
			if keptTrack.Uri == track.Uri {
				if duplicateIndex++; duplicateIndex == needIndex {
					newPlist[i] = keptTrack
					found[track.Uri] = needIndex
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

func (kpr *PlaylistMetaKeeper) Insert(pos int, tracks ...Track) error {
	meta := make([]TrackMeta, len(tracks))
	for i := range tracks {
		meta[i] = TrackMeta{QueuedBy: "user"}
	}
	return kpr.InsertWithMeta(pos, tracks, meta)
}

func (kpr *PlaylistMetaKeeper) Move(fromPos, toPos int) error {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if kpr.meta == nil {
		if err := kpr.update(); err != nil {
			return err
		}
	}
	if err := kpr.Playlist.Move(fromPos, toPos); err != nil {
		return err
	}

	if fromPos >= len(kpr.meta) || toPos >= len(kpr.meta) {
		return fmt.Errorf("Move positions out of range: (%v -> %v) len=%v", fromPos, toPos, len(kpr.meta))
	}
	delta := 0
	if fromPos > toPos {
		delta = 1
	}
	track := kpr.tracks[fromPos]
	kpr.tracks = append(kpr.tracks[:fromPos], kpr.tracks[fromPos+1:]...)
	kpr.tracks = append(kpr.tracks[:toPos+delta], append([]Track{track}, kpr.tracks[toPos+delta:]...)...)
	meta := kpr.meta[fromPos]
	kpr.meta = append(kpr.meta[:fromPos], kpr.meta[fromPos+1:]...)
	kpr.meta = append(kpr.meta[:toPos+delta], append([]TrackMeta{meta}, kpr.meta[toPos+delta:]...)...)
	return nil
}

func (kpr *PlaylistMetaKeeper) Remove(positions ...int) error {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if kpr.meta == nil {
		if err := kpr.update(); err != nil {
			return err
		}
	}
	sort.Ints(positions)
	if err := kpr.Playlist.Remove(positions...); err != nil {
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

func (kpr *PlaylistMetaKeeper) Tracks() ([]Track, error) {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if err := kpr.update(); err != nil {
		return nil, err
	}
	return kpr.tracks, nil
}

func (kpr *PlaylistMetaKeeper) InsertWithMeta(pos int, tracks []Track, meta []TrackMeta) error {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if kpr.meta == nil {
		if err := kpr.update(); err != nil {
			return err
		}
	}
	if err := kpr.Playlist.Insert(pos, tracks...); err != nil {
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

func (kpr *PlaylistMetaKeeper) Meta() ([]TrackMeta, error) {
	kpr.metaLock.Lock()
	defer kpr.metaLock.Unlock()
	if err := kpr.update(); err != nil {
		return nil, err
	}
	return kpr.meta, nil
}
