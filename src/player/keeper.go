package player

import (
	"fmt"
	"sort"
	"sync"
)

// The PlaylistKeeper wraps a Playlist which does not track the meta
// information stored in a PlaylistTrack. This wrapper should be returned by
// players if these do not implement a proper system to store info specific to
// tracks in a playlist.
//
// Any operation performed on this playlist is propagated to the wrapped
// playlist and are safe for concurrent use.
type PlaylistKeeper struct {
	Playlist

	lock sync.Mutex
	meta []PlaylistTrack
}

func (kpr *PlaylistKeeper) Insert(pos int, tracks ...PlaylistTrack) error {
	kpr.lock.Lock()
	defer kpr.lock.Unlock()
	if kpr.meta == nil {
		kpr.Tracks()
	}
	if err := kpr.Playlist.Insert(pos, tracks...); err != nil {
		return err
	}

	if pos == -1 {
		kpr.meta = append(kpr.meta, tracks...)
	} else {
		kpr.meta = append(kpr.meta[:pos], append(tracks, kpr.meta[pos:]...)...)
	}
	return nil
}

func (kpr *PlaylistKeeper) Move(fromPos, toPos int) error {
	kpr.lock.Lock()
	defer kpr.lock.Unlock()
	if kpr.meta == nil {
		kpr.Tracks()
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
	track := kpr.meta[fromPos]
	kpr.meta = append(kpr.meta[:fromPos], kpr.meta[fromPos+1:]...)
	kpr.meta = append(kpr.meta[:toPos+delta], append([]PlaylistTrack{track}, kpr.meta[toPos+delta:]...)...)
	return nil
}

func (kpr *PlaylistKeeper) Remove(positions ...int) error {
	kpr.lock.Lock()
	defer kpr.lock.Unlock()
	if kpr.meta == nil {
		kpr.Tracks()
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
		kpr.meta = append(kpr.meta[:pos], kpr.meta[pos+1:]...)
	}
	return nil
}

func (kpr *PlaylistKeeper) Tracks() ([]PlaylistTrack, error) {
	kpr.lock.Lock()
	defer kpr.lock.Unlock()

	tracks, err := kpr.Playlist.Tracks()
	if err != nil {
		return nil, err
	}

	inferDefault := func(target, source *PlaylistTrack) {
		if source.Progress != 0 {
			target.Progress = source.Progress
		}
		if target.QueuedBy == "" {
			if source.QueuedBy != "" {
				target.QueuedBy = source.QueuedBy
			} else {
				target.QueuedBy = "user"
			}
		}
	}

	newPlist := make([]PlaylistTrack, len(tracks))
	found := map[string]int{}
outer:
	for i, track := range tracks {
		needIndex := found[track.Uri] + 1
		duplicateIndex := 0
		for _, keptTrack := range kpr.meta {
			if keptTrack.Uri == track.Uri {
				if duplicateIndex++; duplicateIndex == needIndex {
					newPlist[i] = keptTrack
					found[track.Uri] = needIndex
					inferDefault(&newPlist[i], &track)
					continue outer
				}
			}
		}

		plTrack := &newPlist[i]
		plTrack.Track = track.Track
		inferDefault(plTrack, &track)
	}
	kpr.meta = newPlist
	return newPlist, nil
}
