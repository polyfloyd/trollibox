package library

import (
	"io"

	"github.com/polyfloyd/trollibox/src/util"
)

// A Library is a database that is able to recall tracks that can be played.
type Library interface {
	// The following events are emitted:
	//   "tracks" After the track library was changed.
	util.Eventer

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
