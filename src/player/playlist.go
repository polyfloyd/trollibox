package player

type Playlist interface {
	// Insert a bunch of tracks into the playlist starting at the specified
	// position. Position -1 can be used to append to the end of the playlist.
	Insert(pos int, track ...PlaylistTrack) error

	// Moves a track from position A to B. An error is returned if at least one
	// of the positions is out of range.
	Move(fromPos, toPos int) error

	// Removes one or more tracks from the playlist.
	Remove(pos ...int) error

	// Returns all tracks in the playlist.
	Tracks() ([]PlaylistTrack, error)
}

type TrackIterator interface {
	// Returns the next track from the iterator. If the bool flag is false, the
	// iterator has reached the end.
	NextTrack() (PlaylistTrack, bool)
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
				plist, currentTrackIndex, err := pl.Playlist()
				if err != nil {
					com <- err
					return
				}
				state, err := pl.State()
				if err != nil {
					com <- err
					return
				}
				if state != PlayStateStopped && currentTrackIndex != -1 {
					continue
				}

				track, ok := iter.NextTrack()
				if !ok {
					break outer
				}
				if err := plist.Insert(-1, track); err != nil {
					com <- err
					return
				}
				tracks, err := plist.Tracks()
				if err != nil {
					com <- err
					return
				}
				pl.SetState(PlayStatePlaying)
				pl.Seek(len(tracks)-1, -1)

			case <-com:
				break outer
			}
		}
		com <- nil
	}()

	return com
}
