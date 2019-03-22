package player

import (
	"time"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/util"
)

// PlayState enumerates all 3 possible states of playback.
type PlayState string

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

type (
	// Event is the type of event emitted by the player.
	Event interface{}
	// PlaylistEvent is emitted after the playlist or the current playlist was
	// changed.
	PlaylistEvent struct {
		Index int
	}
	// PlayStateEvent is emitted after the playstate was changed.
	PlayStateEvent struct {
		State PlayState
	}
	// TimeEvent is emitted after the playback offset of the currently playing
	// track was changed.
	TimeEvent struct {
		Time time.Duration
	}
	// VolumeEvent is emitted after the volume was changed.
	VolumeEvent struct {
		Volume int
	}
	// ListEvent is emitted after a stored playlist was changed.
	ListEvent struct{}
	// AvailabilityEvent is emitted after the player comes online or goes
	// offline.
	AvailabilityEvent struct {
		Available bool
	}
)

// The Player is the heart of Trollibox. This interface provides all common
// actions that can be performed on a mediaplayer.
type Player interface {
	// Any type of player.Event may be emitted.
	//
	// NOTE: The library.UpdateEvent is also emitted. This is legacy behaviour
	// and should be removed in the future.
	util.Eventer

	// It is common for backends to also have some kind of track library.
	// Players should therefore return an implementation of the respective
	// interface.
	Library() library.Library

	// Returns the currently playing playlist.
	Playlist() MetaPlaylist

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

	// Gets the set volume as a value between 0 and 100.
	Volume() (int, error)

	// Sets the volume of the player. The volume should be updated even when
	// nothing is playing. The value is clamped between 0 and 100.
	SetVolume(vol int) error

	// Retrieves the custom finite playlists that are stored by the player and
	// maps them by their unique name.
	Lists() (map[string]Playlist, error)

	// Reports wether the player is online and reachable.
	Available() bool
}
