package player

import (
	"io"
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

type PlaylistTrack struct {
	TrackIdentity
	Progress time.Duration
	QueuedBy string
}

type Player interface {
	// Gets information about the specified tracks. If no identities are given, all
	// tracks in the player's libary are returned. Otherwise, information about
	// the specified track is returned.
	TrackInfo(identites ...TrackIdentity) ([]Track, error)

	// Returns the tracks in the playlist of this player. The track at index 0
	// is the currently playing track.
	Playlist() ([]PlaylistTrack, error)

	// Updates the player's playlist. Changing the first track will cause the
	// player to start playing the first track in the new playlist. Changing
	// the progress of the first track has no effect on the currently playing
	// track.
	SetPlaylist(plist []PlaylistTrack) error

	// Seeks to the absolute point in time of the currently playing track. This
	// is a no-op if player has been stopped.
	Seek(offset time.Duration) error

	// Abort playback of the currently playing track and start playing the next
	// one. If the current track is the last track of the queue, the playstate
	// is set to stopped.
	Next() error

	State() (PlayState, error)

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
	//   "seek"         After the playback offset of the currently playing track was changed.
	//   "tracks"       After the track library was changed.
	//   "volume"       After the volume was changed.
	//   "availability" After the player comes online or goes offline.
	Events() *util.Emitter
}

type TrackIdentity interface {
	Uri() string
}

type Track interface {
	TrackIdentity

	Artist() string
	Title() string
	Genre() string
	Album() string
	AlbumArtist() string
	AlbumTrack() string
	AlbumDisc() string
	Duration() time.Duration

	// Returns the artwork for this track as a reader of image data along with
	// its MIME type. The caller is responsible for closing the reader.
	Art() (image io.ReadCloser, mime string)
}

// Get an attribute of a track by its name. Accepted names are:
//   "id" (alias for "uri")
//   "uri"
// If the track implements the Track interface:
//   "artist"
//   "title"
//   "genre"
//   "album"
//   "albumartist"
//   "albumtrack"
//   "albumdisc"
//   "duration"
func TrackAttr(trackId TrackIdentity, attr string) interface{} {
	switch attr {
	case "id":
		fallthrough
	case "uri":
		return trackId.Uri()
	}

	if track, ok := trackId.(Track); ok {
		switch attr {
		case "artist":
			return track.Artist()
		case "title":
			return track.Title()
		case "genre":
			return track.Genre()
		case "album":
			return track.AlbumArtist()
		case "albumartist":
			return track.AlbumArtist()
		case "albumtrack":
			return track.AlbumTrack()
		case "albumdisc":
			return track.AlbumDisc()
		case "duration":
			return track.Duration()
		}
	}

	return nil
}

type trackID string

func (tr trackID) Uri() string {
	return string(tr)
}

func TrackIdentities(uris ...string) []TrackIdentity {
	tracks := make([]TrackIdentity, len(uris))
	for i, uri := range uris {
		tracks[i] = trackID(uri)
	}
	return tracks
}

// Convenience method for appending to a track to the playlist of a player.
func PlaylistAppend(pl Player, tracks ...PlaylistTrack) error {
	plist, err := pl.Playlist()
	if err != nil {
		return err
	}
	return pl.SetPlaylist(append(plist, tracks...))
}

// Convenience method for setting the playlist using just the ids. The metadata
// is reconstructed using InterpolatePlaylistMeta(). It's probably best to not
// use this function. Instead, keep track of the metadata.
func SetPlaylistIds(pl Player, ids []TrackIdentity) error {
	plist, err := pl.Playlist()
	if err != nil {
		return err
	}
	return pl.SetPlaylist(InterpolatePlaylistMeta(plist, ids))
}

// Attempts to get the queuedby and progress information from the player's
// playlist and applies it to the supplied id list.
func InterpolatePlaylistMeta(plist []PlaylistTrack, ids []TrackIdentity) []PlaylistTrack {
	newPlist := make([]PlaylistTrack, len(ids))

	found := map[string]int{}
outer:
	for i, id := range ids {
		needIndex := found[id.Uri()] + 1
		duplicateIndex := 0

		for _, tr := range plist {
			if tr.Uri() == id.Uri() {
				if duplicateIndex++; duplicateIndex == needIndex {
					newPlist[i] = tr
					found[id.Uri()] = needIndex
					continue outer
				}
			}
		}

		newPlist[i] = PlaylistTrack{
			TrackIdentity: id,
			Progress:      0,
			QueuedBy:      "user",
		}
	}

	return newPlist
}
