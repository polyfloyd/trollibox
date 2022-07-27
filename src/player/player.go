package player

import (
	"context"
	"errors"
	"time"

	"trollibox/src/library"
	"trollibox/src/util"
)

// ErrUnavailable is returned from functions that operate on player state when
// a player unreachable for any reason.
var ErrUnavailable = errors.New("the player is not available")

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
		TrackIndex int
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
	Playlist() Playlist[MetaTrack]

	Status(context.Context) (*Status, error)

	// SetTime Seeks to the absolute point in time of the current track. This is a
	// no-op if player has been stopped.
	SetTime(context.Context, time.Duration) error

	// Jumps to the specified track in the players' playlist. If the index is
	// bigger than the length of the playlist, the playlist is ended and the
	// state is setted to stopped.
	SetTrackIndex(context.Context, int) error

	// Signal the player to start/resume, stop or pause playback. If the
	// playlist is empty, a playlist-end event is emitted.
	SetState(context.Context, PlayState) error

	// Sets the volume of the player. The volume should be updated even when
	// nothing is playing. The value is clamped between 0 and 100.
	SetVolume(context.Context, int) error

	// Retrieves the custom finite playlists that are stored by the player and
	// maps them by their unique name.
	Lists(context.Context) (map[string]Playlist[library.Track], error)
}

type Status struct {
	// The absolute index into the players' playlist.
	TrackIndex int
	// The time offset into the currently playing track. 0 if no track is being played.
	Time time.Duration
	// The current playstate of the player.
	PlayState PlayState
	// The set volume as a value between 0 and 100.
	Volume int
}
