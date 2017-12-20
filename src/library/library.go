package library

import (
	"io"

	"github.com/polyfloyd/trollibox/src/util"
)

type UpdateEvent struct{}

// A Library is a database that is able to recall tracks that can be played.
type Library interface {
	// An UpdateEvent may be emitted after the track library was changed.
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
	// Request track information from all libraries in parallel.
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
	// Wait for all all lookups to complete by receiving the result from all
	// channels. Each element in accumTracks is the result from each library.
	accumTracks := make([][]Track, 0, len(libs))
	for _, result := range accumChannels {
		switch t := (<-result).(type) {
		case error:
			return nil, t
		case []Track:
			accumTracks = append(accumTracks, t)
		default:
			panic("UNREACHABLE")
		}
	}

	// Flatten the result by picking the tracks with the lowest index from all
	// results.
	tracks := make([]Track, len(uris))
	for _, libraryResult := range accumTracks {
		for index, tr := range libraryResult {
			if tr.URI != "" && tracks[index].URI == "" {
				tracks[index] = tr
			}
		}
	}
	return tracks, nil
}
