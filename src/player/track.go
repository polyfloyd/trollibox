package player

import (
	"regexp"
	"strings"
	"time"
)

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

// Get an attribute of a track by its name. Accepted names are:
//   "uri"
//   "id" (alias for "uri", deprecated)
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

type PlaylistTrack struct {
	Track
	Progress time.Duration
	QueuedBy string
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
