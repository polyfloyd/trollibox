package player

import (
	"context"
	"trollibox/src/library"
)

type PlaylistTrack interface {
	library.Track | MetaTrack

	GetURI() string
}

// A Playlist is a mutable ordered collection of tracks.
type Playlist[T PlaylistTrack] interface {
	// Insert a bunch of tracks into the playlist starting at the specified
	// position. Position -1 can be used to append to the end of the playlist.
	Insert(ctx context.Context, pos int, tracks ...T) error

	// Moves a track from position A to B. An error is returned if at least one
	// of the positions is out of range.
	Move(ctx context.Context, fromPos, toPos int) error

	// Removes one or more tracks from the playlist.
	Remove(ctx context.Context, pos ...int) error

	// Returns all tracks in the playlist.
	Tracks(ctx context.Context) ([]T, error)

	// Len() returns the total number of tracks in the playlist. It's much the
	// same as getting the length of the slice returned by Tracks(), but
	// probably a lot faster.
	Len(ctx context.Context) (int, error)
}

// MetaTrack is a track that is queued in the playlist of a player.
//
// It augments a regular library track with fields that are valid while the track is queued.
type MetaTrack struct {
	library.Track
	// QueuedBy indicates by what entity a track was added.
	// Can be either "user" or "system".
	QueuedBy string
}

// A TrackIterator is a type that produces a finite or infinite stream of tracks.
//
// Used by AutoAppend.
type TrackIterator interface {
	// Returns the next track from the iterator. If the bool flag is false, the
	// iterator has reached the end. The player that is requesting the next
	// track is specified.
	NextTrack(ctx context.Context, lib library.Library) (MetaTrack, bool)
}

// AutoAppend attaches a listener to the specified player. The iterator is used
// to get tracks which are played when the playlist of the player runs out.
//
// Sending a value over the returned channel interrupts the operation.
// Receiving from the channel blocks until no more tracks are available from
// the iterator or an error is encountered.
func AutoAppend(pl Player, iter TrackIterator, cancel <-chan struct{}) <-chan error {
	ctx := context.Background()
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
				status, err := pl.Status(ctx)
				if err != nil {
					errc <- err
					return
				}
				if status.PlayState != PlayStateStopped && status.TrackIndex != -1 {
					continue
				}

				metaTrack, ok := iter.NextTrack(ctx, pl.Library())
				if !ok {
					break outer
				}
				if err := plist.Insert(ctx, -1, metaTrack); err != nil {
					errc <- err
					return
				}
				plistLen, err := plist.Len(ctx)
				if err != nil {
					errc <- err
					return
				}
				if err := pl.SetState(ctx, PlayStatePlaying); err != nil {
					errc <- err
					return
				}
				if err := pl.SetTrackIndex(ctx, plistLen-1); err != nil {
					errc <- err
					return
				}

			case <-cancel:
				break outer
			}
		}
	}()
	return errc
}
