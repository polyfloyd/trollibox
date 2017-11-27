package player

import (
	"io"
	"sync"
	"time"

	"github.com/polyfloyd/trollibox/src/util"
)

const (
	PlayStateInvalid = PlayState("")
	PlayStatePlaying = PlayState("playing")
	PlayStateStopped = PlayState("stopped")
	PlayStatePaused  = PlayState("paused")
)

type PlayState string

type Library interface {
	// Returns all available tracks in the library.
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
			if tr.URI != "" && tracks[j].URI == "" {
				tracks[j] = tr
			}
		}
	}
	return tracks, nil
}

type Player interface {
	Library

	// Gets the time offset into the currently playing track. 0 if no track is
	// being played.
	Time() (time.Duration, error)

	// SetTime Seeks to the absolute point in time of the current track. This is a
	// no-op if player has been stopped.
	SetTime(offset time.Duration) error

	// Returns absolute index into the players' playlist.
	TrackIndex() (int, error)

	// Jumps to the specified track in the players' playlist. If the index is
	// bigger than the length of the playlist, the playlist is ended.
	SetTrackIndex(trackIndex int) error

	// Returns the current playstate of the player.
	State() (PlayState, error)

	// Signal the player to start/resume, stop or pause playback. If the
	// playlist is empty, a playlist-end event is emitted.
	SetState(state PlayState) error

	// Gets the set volume as a uniform float value between 0 and 1.
	Volume() (float32, error)

	// Sets the volume of the player. The volume should be updated even when
	// nothing is playing.
	SetVolume(vol float32) error

	// Retrieves the custom finite playlists that are stored by the player and
	// maps them by their unique name.
	Lists() (map[string]Playlist, error)

	// Reports wether the player is online and reachable.
	Available() bool

	// Returns the currently playing playlist.
	Playlist() MetaPlaylist

	// Gets the event emitter for this player. The following events are emitted:
	//   "playlist"     After the playlist or the current playlists' was changed.
	//   "playstate"    After the playstate was changed.
	//   "time"         After the playback offset of the currently playing track was changed.
	//   "volume"       After the volume was changed.
	//   "list"         After a stored playlist was changed.
	//   "tracks"       After the track library was changed.
	//   "availability" After the player comes online or goes offline.
	Events() *util.Emitter
}
