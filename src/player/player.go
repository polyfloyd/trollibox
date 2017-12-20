package player

import (
	"time"

	"github.com/polyfloyd/trollibox/src/library"
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

// Event is the type of event emitted by the player.
type Event string

const (
	// After the playlist or the current playlists' was changed.
	PlaylistEvent = Event("playlist")
	// After the playstate was changed.
	PlaystateEvent = Event("playstate")
	// After the playback offset of the currently playing track was changed.
	TimeEvent = Event("time")
	// After the volume was changed.
	VolumeEvent = Event("volume")
	// After a stored playlist was changed.
	ListEvent = Event("list")
	// After the player comes online or goes offline.
	AvailabilityEvent = Event("availability")
)

// The Player is the heart of Trollibox. This interface provides all common
// actions that can be performed on a mediaplayer.
type Player interface {
	// It is common for backends to also have some kind of track library.
	// Players should therefore implement the respective interface.
	//
	// Through Library, the util.Eventer interface is also required. Any type
	// of player.Event may be emitted.
	library.Library

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
}
