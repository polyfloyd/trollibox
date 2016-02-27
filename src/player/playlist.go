package player

type Playlist interface {
	// Insert a bunch of tracks into the playlist starting at the specified
	// position. Position -1 can be used to append to the end of the playlist.
	Insert(pos int, tracks ...Track) error

	// Moves a track from position A to B. An error is returned if at least one
	// of the positions is out of range.
	Move(fromPos, toPos int) error

	// Removes one or more tracks from the playlist.
	Remove(pos ...int) error

	// Returns all tracks in the playlist.
	Tracks() ([]Track, error)

	// Len() returns the total number of tracks in the playlist.
	Len() (int, error)
}

// A MetaPlaylist is used as the main playlist of a player. It allows metadata
// specific to tracks in the playlist to be persisted.
type MetaPlaylist interface {
	Playlist

	InsertWithMeta(pos int, tracks []Track, meta []TrackMeta) error

	Meta() ([]TrackMeta, error)
}

type TrackIterator interface {
	// Returns the next track from the iterator. If the bool flag is false, the
	// iterator has reached the end.
	NextTrack() (Track, TrackMeta, bool)
}

// Attaches a listener to the specified player. The iterator is used to get
// tracks which are played when the playlist of the player runs out.
//
// Sending a value over the returned channel interrupts the operation.
// Receiving from the channel blocks until no more tracks are available from
// the iterator or an error is encountered.
func AutoAppend(pl Player, iter TrackIterator) chan error {
	com := make(chan error, 1)

	go func() {
		events := pl.Events().Listen()
		defer pl.Events().Unlisten(events)
		defer close(com)
	outer:
		for {
			select {
			case event := <-events:
				if event != "playstate" && event != "playlist" {
					continue
				}
				plist := pl.Playlist()
				trackIndex, err := pl.TrackIndex()
				if err != nil {
					com <- err
					return
				}
				state, err := pl.State()
				if err != nil {
					com <- err
					return
				}
				if state != PlayStateStopped && trackIndex != -1 {
					continue
				}

				track, meta, ok := iter.NextTrack()
				if !ok {
					break outer
				}
				if err := plist.InsertWithMeta(-1, []Track{track}, []TrackMeta{meta}); err != nil {
					com <- err
					return
				}
				tracks, err := plist.Tracks()
				if err != nil {
					com <- err
					return
				}
				pl.SetState(PlayStatePlaying)
				pl.SetTrackIndex(len(tracks) - 1)

			case <-com:
				break outer
			}
		}
		com <- nil
	}()

	return com
}