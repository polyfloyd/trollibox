package player

import (
	"io"
	"sync"
	"time"

	"../util"
)

const (
	PlayStateInvalid = PlayState(iota)
	PlayStatePlaying
	PlayStateStopped
	PlayStatePaused
)

type PlayState int

func NamedPlaystate(str string) PlayState {
	switch str {
	case "playing":
		return PlayStatePlaying
	case "stopped":
		return PlayStateStopped
	case "paused":
		return PlayStatePaused
	default:
		return PlayStateInvalid
	}
}

func (state PlayState) Name() string {
	switch state {
	case PlayStatePlaying:
		return "playing"
	case PlayStateStopped:
		return "stopped"
	case PlayStatePaused:
		return "paused"
	default:
		return "invalid"
	}
}

type Library interface {
	// Returns all available tracks in the libary.
	Tracks() ([]Track, error)

	// Gets information about the specified tracks. If a track is not found, a
	// zero track is returned at that index.
	TrackInfo(uris ...string) ([]Track, error)

	// Returns the artwork for the track as a reader of image data along with
	// its MIME type. The caller is responsible for closing the reader.
	TrackArt(uri string) (image io.ReadCloser, mime string)
}

// Looks for the track information in all the libraries supplied. If the track
// is found in more than one library, precedence is given to the library at the
// lowest index.
func AllTrackInfo(libs []Library, uris ...string) ([]Track, error) {
	done := make(chan struct{})
	defer close(done)
	errs := make(chan error)
	defer close(errs)
	var errorred bool
	var chanLock sync.Mutex

	accumTracks := make([][]Track, len(libs))
	for i, lib := range libs {
		go func(tracksPtr *[]Track, lib Library) {
			tracks, err := lib.TrackInfo(uris...)
			chanLock.Lock()
			defer chanLock.Unlock()
			if errorred {
				return
			}
			if err != nil {
				errs <- err
				return
			}
			*tracksPtr = tracks
			done <- struct{}{}
		}(&accumTracks[i], lib)
	}

	for range libs {
		select {
		case err := <-errs:
			chanLock.Lock()
			errorred = true
			chanLock.Unlock()
			return nil, err
		case <-done:
		}
	}

	tracks := make([]Track, len(uris))
	for i := range libs {
		for j, tr := range accumTracks[i] {
			if tr.Uri != "" && tracks[j].Uri == "" {
				tracks[j] = tr
			}
		}
	}
	return tracks, nil
}

type Player interface {
	Library

	// Returns the currently playing playlist as well as the index of the
	// currently playing track.
	Playlist() (plist Playlist, currentTrackIndex int, err error)

	// Seeks to the absolute point in time of the specified track. This
	// is a no-op if player has been stopped. Use -1 as trackIndex to seek in
	// the current track.
	Seek(trackIndex int, offset time.Duration) error

	State() (PlayState, error)

	// Signal the player to start/resume, stop or pause playback. If the
	// playlist is empty, a playlist-end event is emitted.
	SetState(state PlayState) error

	// Gets the set volume as a uniform float value between 0 and 1.
	Volume() (float32, error)

	// Sets the volume of the player. The volume should be updated even when
	// nothing is playing.
	SetVolume(vol float32) error

	// Reports wether the player is online and reachable.
	Available() bool

	// Gets the event emitter for this player. The following events are emitted:
	//   "playlist"     After the playlist was changed. Includes changes to the
	//                  currently playing track.
	//   "playlist-end" After the playlist has ended or an attempt was made to
	//                  play a track when no more tracks are available for playing.
	//   "playstate"    After the playstate was changed.
	//   "progress"     After the playback offset of the currently playing track was changed.
	//   "tracks"       After the track library was changed.
	//   "volume"       After the volume was changed.
	//   "availability" After the player comes online or goes offline.
	Events() *util.Emitter
}
