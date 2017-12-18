package player

import (
	"io"
	"time"

	"github.com/polyfloyd/trollibox/src/util"
)

const (
	// PlayStateInvalid is the zero value, which is invalid.
	PlayStateInvalid = PlayState("")
	// PlayStatePlaying represents the state of a player that is playing a
	// track.
	PlayStatePlaying = PlayState("playing")
	// PlayStateStopped represents the state of a player that is playing
	// nothing.
	PlayStateStopped = PlayState("stopped")
	// PlayStatePaused represents the state of a player has loaded a track but
	// is not progressing playback.
	PlayStatePaused = PlayState("paused")
)

// PlayState enumerates all 3 possible states of playback.
type PlayState string

// A Library is a database that is able to recall tracks that can be played.
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

// AllTrackInfo looks for the track information in all the libraries supplied.
//
// If the track is found in more than one library, precedence is given to the
// library at the lowest index.
func AllTrackInfo(libs []Library, uris ...string) ([]Track, error) {
	accumChannels := make([]<-chan interface{}, 0, len(libs))
	for _, lib := range libs {
		ch := make(chan interface{})
		go func(lib Library) {
			defer close(ch)
			tracks, err := lib.TrackInfo(uris...)
			if err != nil {
				ch <- err
			} else {
				ch <- tracks
			}
		}(lib)
		accumChannels = append(accumChannels, ch)
	}
	accumTracks := make([][]Track, 0, len(libs))
	for _, result := range accumChannels {
		switch t := (<-result).(type) {
		case error:
			return nil, t
		case []Track:
			accumTracks = append(accumTracks, t)
		}
	}

	tracks := make([]Track, len(uris))
	for _, accum := range accumTracks {
		for j, tr := range accum {
			if tr.URI != "" && tracks[j].URI == "" {
				tracks[j] = tr
				break
			}
		}
	}
	return tracks, nil
}

// The Player is the heart of Trollibox. This interface provides all common
// actions that can be performed on a mediaplayer.
type Player interface {
	// It is common for backends to also have some kind of track library.
	// Players should therefore implement the respective interface.
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
	// bigger than the length of the playlist, the playlist is ended and the
	// state is setted to stopped.
	SetTrackIndex(trackIndex int) error

	// Returns the current playstate of the player.
	State() (PlayState, error)

	// Signal the player to start/resume, stop or pause playback. If the
	// playlist is empty, a playlist-end event is emitted.
	SetState(state PlayState) error

	// Gets the set volume as a uniform float value between 0 and 1.
	Volume() (float32, error)

	// Sets the volume of the player. The volume should be updated even when
	// nothing is playing. The value is clamped between 0 and 1.
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
