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
