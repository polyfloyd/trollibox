package player

import (
	"github.com/polyfloyd/trollibox/src/library"
)

// A Playlist is a mutable ordered collection of tracks.
type Playlist interface {
	// Insert a bunch of tracks into the playlist starting at the specified
	// position. Position -1 can be used to append to the end of the playlist.
	Insert(pos int, tracks ...library.Track) error

	// Moves a track from position A to B. An error is returned if at least one
	// of the positions is out of range.
	Move(fromPos, toPos int) error

	// Removes one or more tracks from the playlist.
	Remove(pos ...int) error

	// Returns all tracks in the playlist.
	Tracks() ([]library.Track, error)

	// Len() returns the total number of tracks in the playlist. It's much the
	// same as getting the length of the slice returned by Tracks(), but
	// probably a lot faster.
	Len() (int, error)
}

// A MetaPlaylist is used as the main playlist of a player. It allows metadata
// specific to tracks in the playlist to be persisted.
type MetaPlaylist interface {
	Playlist

	InsertWithMeta(pos int, tracks []library.Track, meta []TrackMeta) error

	Meta() ([]TrackMeta, error)
}

// A TrackIterator is a type that produces a finite or infinite stream of tracks.
//
// Used by AutoAppend.
type TrackIterator interface {
	// Returns the next track from the iterator. If the bool flag is false, the
	// iterator has reached the end. The player that is requesting the next
	// track is specified.
	NextTrack(lib library.Library) (library.Track, TrackMeta, bool)
}

// AutoAppend attaches a listener to the specified player. The iterator is used
// to get tracks which are played when the playlist of the player runs out.
//
// Sending a value over the returned channel interrupts the operation.
// Receiving from the channel blocks until no more tracks are available from
// the iterator or an error is encountered.
func AutoAppend(pl Player, iter TrackIterator, cancel <-chan struct{}) <-chan error {
	errc := make(chan error, 1)
	go func() {
		events := pl.Events().Listen()
		defer pl.Events().Unlisten(events)
		defer close(errc)
	outer:
		for {
			select {
			case event := <-events:
				_, okA := event.(PlayStateEvent)
				_, okB := event.(PlaylistEvent)
				if !okA && !okB {
					continue
				}

				plist := pl.Playlist()
				trackIndex, err := pl.TrackIndex()
				if err != nil {
					errc <- err
					return
				}
				state, err := pl.State()
				if err != nil {
					errc <- err
					return
				}
				if state != PlayStateStopped && trackIndex != -1 {
					continue
				}

				track, meta, ok := iter.NextTrack(pl.Library())
				if !ok {
					break outer
				}
				if err := plist.InsertWithMeta(-1, []library.Track{track}, []TrackMeta{meta}); err != nil {
					errc <- err
					return
				}
				plistLen, err := plist.Len()
				if err != nil {
					errc <- err
					return
				}
				pl.SetState(PlayStatePlaying)
				pl.SetTrackIndex(plistLen - 1)

			case <-cancel:
				break outer
			}
		}
	}()
	return errc
}
