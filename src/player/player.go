package player

import (
	"io"

	"./event"
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

const (
	PlayStateInvalid = iota
	PlayStatePlaying
	PlayStateStopped
	PlayStatePaused
)

type Player interface {
	ListTracks(path string, recursive bool) ([]Track, error)

	Queue(track TrackIdentity, queuedBy string) error

	// Gets the set volume as a uniform float.
	Volume() (float32, error)

	SetVolume(vol float32) error

	// Retrieves a list of all tracks in the playlist. The track at index 0 is
	// the track that is being played.
	Playlist() ([]PlaylistTrack, error)

	SetPlaylist(tracks []TrackIdentity) error

	Seek(seconds int) error

	Next() error

	State() (PlayState, error)

	SetState(state PlayState) error

	Events() *event.Emitter
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
	Duration() int
	Art() (image io.ReadCloser, mime string)
}

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

	if plTrack, ok := trackId.(PlaylistTrack); ok {
		switch attr {
		case "queuedby":
			return plTrack.QueuedBy()
		case "progress":
			return plTrack.Progress()
		}
	}

	return nil
}

type PlaylistTrack interface {
	Track

	QueuedBy() string
	Progress() int
}

type trackIdentity string

func (tr trackIdentity) Uri() string {
	return string(tr)
}

func TrackIdentities(uris []string) []TrackIdentity {
	tracks := make([]TrackIdentity, len(uris))
	for i, uri := range uris {
		tracks[i] = trackIdentity(uri)
	}
	return tracks
}
