package player

import (
	"io"
	"regexp"
	"strings"
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

type Library interface {
	// Returns all available tracks in the libary.
	Tracks() ([]Track, error)

	// Gets information about the specified tracks. If a track is not found, a
	// zero track is returned at that index.
	TrackInfo(identites ...TrackIdentity) ([]Track, error)

	// Returns the artwork for the track as a reader of image data along with
	// its MIME type. The caller is responsible for closing the reader.
	TrackArt(track TrackIdentity) (image io.ReadCloser, mime string)
}

type Player interface {
	Library

	// Returns the tracks in the playlist of this player. The track at index 0
	// is the currently playing track.
	Playlist() ([]PlaylistTrack, error)

	// Updates the player's playlist. Changing the first track will cause the
	// player to start playing the first track in the new playlist.
	//
	// Changing the progress of the first track has no effect on the currently
	// playing track, use Seek() instead.
	//
	// If the new playlist contains at least one track, the player starts
	// playing the first track. Otherwise, a playlist-end event is emitted.
	SetPlaylist(plist []PlaylistTrack) error

	// Seeks to the absolute point in time of the currently playing track. This
	// is a no-op if player has been stopped.
	Seek(offset time.Duration) error

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

type Track struct {
	Uri         string        `json:"uri"`
	Artist      string        `json:"artist,omitempty"`
	Title       string        `json:"title,omitempty"`
	Genre       string        `json:"genre,omitempty"`
	Album       string        `json:"album,omitempty"`
	AlbumArtist string        `json:"albumartist,omitempty"`
	AlbumTrack  string        `json:"albumtrack,omitempty"`
	AlbumDisc   string        `json:"albumdisc,omitempty"`
	Duration    time.Duration `json:"duration"`
	HasArt      bool          `json:"hasart"`
}

func (track Track) TrackUri() string {
	return track.Uri
}

type TrackIdentity interface {
	TrackUri() string
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
func (track *Track) Attr(attr string) interface{} {
	switch attr {
	case "id":
		fallthrough
	case "uri":
		return track.Uri
	case "artist":
		return track.Artist
	case "title":
		return track.Title
	case "genre":
		return track.Genre
	case "album":
		return track.Album
	case "albumartist":
		return track.AlbumArtist
	case "albumtrack":
		return track.AlbumTrack
	case "albumdisc":
		return track.AlbumDisc
	case "duration":
		return track.Duration
	case "hasart":
		return track.HasArt
	}
	return nil
}

func TrackIdentities(uris ...string) []TrackIdentity {
	tracks := make([]TrackIdentity, len(uris))
	for i, uri := range uris {
		tracks[i] = Track{Uri: uri}
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

// Abort playback of the currently playing track and start playing the next
// one.
func PlaylistNext(pl Player) error {
	plist, err := pl.Playlist()
	if err != nil {
		return err
	}
	if len(plist) > 0 {
		if err := pl.SetPlaylist(plist[1:]); err != nil {
			return err
		}
	} else {
		if err := pl.SetPlaylist([]PlaylistTrack{}); err != nil {
			return err
		}
	}
	return nil
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
		needIndex := found[id.TrackUri()] + 1
		duplicateIndex := 0

		for _, tr := range plist {
			if tr.TrackUri() == id.TrackUri() {
				if duplicateIndex++; duplicateIndex == needIndex {
					newPlist[i] = tr
					found[id.TrackUri()] = needIndex
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

// Players may use this function to extract the artist and title from other
// track information if they are unavailable.
func InterpolateMissingFields(track *Track) {
	if strings.HasPrefix(track.Uri, "http") {
		return
	}

	// First, attempt to find an "<artist> - <title>" string in the track title.
	if track.Artist == "" && track.Title != "" {
		re := regexp.MustCompile("(.+)\\s+-\\s+(.+)")
		if match := re.FindStringSubmatch(track.Title); match != nil {
			track.Artist, track.Title = match[0], match[1]
		}
	}

	// Also look for the <artist> - <title> patterin in the filename.
	if track.Artist == "" || track.Title == "" {
		re := regexp.MustCompile("^(?:.*\\/)?(.+)\\s+-\\s+(.+)\\.\\w+$")
		if match := re.FindStringSubmatch(track.Uri); match != nil {
			track.Artist, track.Title = match[1], match[2]
		}
	}

	// Still nothing? Just use the filename or url.
	if track.Title == "" {
		re := regexp.MustCompile("^.*\\/(.+)\\.\\w+$")
		if match := re.FindStringSubmatch(track.Uri); match != nil {
			track.Title = match[1]
		}
	}
}
