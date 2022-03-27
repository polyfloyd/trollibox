package library

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	interpArtistTitleInTitle    = regexp.MustCompile(`(.+)\s+-\s+(.+)`)
	interpArtistTitleInFilename = regexp.MustCompile(`(?:(?:\d+\.\s+)|(?:\d+\s+-\s+))?([^/]+?)\s+-\s+([^/]+)\.\w+$`)
	interpFilename              = regexp.MustCompile(`^.*\/(.+)\.\w+$`)
)

// Track holds all information associated with a single piece of music.
type Track struct {
	URI         string        `json:"uri"`
	Artist      string        `json:"artist,omitempty"`
	Title       string        `json:"title,omitempty"`
	Genre       string        `json:"genre,omitempty"`
	Album       string        `json:"album,omitempty"`
	AlbumArtist string        `json:"albumartist,omitempty"`
	AlbumTrack  string        `json:"albumtrack,omitempty"`
	AlbumDisc   string        `json:"albumdisc,omitempty"`
	Duration    time.Duration `json:"duration"`
	ModTime     time.Time     `json:"-"`
}

// GetURI implements the PlaylistTrack interface.
func (t Track) GetURI() string {
	return t.URI
}

// Attr gets an attribute of a track by its name. Accepted names are:
//   "uri"
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
	case "uri":
		return track.URI
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
		return int64(track.Duration / time.Second)
	}
	return nil
}

func (track Track) String() string {
	return fmt.Sprintf("%s - %s (%v)", track.Artist, track.Title, track.Duration)
}

// InterpolateMissingFields extracts the artist and title from other track
// information if they are unavailable and applies them to the specified track.
//
// Players should use this to homogenize their library.
func InterpolateMissingFields(track *Track) {
	if track.Artist != "" && track.Title != "" {
		return
	}
	if strings.HasPrefix(track.URI, "http") {
		return
	}

	// Attempt to find an "<artist> - <title>" string in the track title.
	if track.Artist == "" && track.Title != "" {
		if match := interpArtistTitleInTitle.FindStringSubmatch(track.Title); match != nil {
			track.Artist, track.Title = match[1], match[2]
			return
		}
	}

	// Look for the "<artist> - <title>" pattern in the filename.
	if track.Artist == "" || track.Title == "" {
		if match := interpArtistTitleInFilename.FindStringSubmatch(track.URI); match != nil {
			track.Artist, track.Title = match[1], match[2]
			return
		}
	}

	// Still nothing? Just use the filename or url.
	if track.Title == "" {
		if match := interpFilename.FindStringSubmatch(track.URI); match != nil {
			track.Title = match[1]
		}
	}
}
