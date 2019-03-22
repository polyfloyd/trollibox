package mpd

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fhs/gompd/mpd"
	log "github.com/sirupsen/logrus"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/library/cache"
	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/util"
)

const uriSchema = "mpd://"

// Event is an event which signals a change in one of MPD's subsystems.
type Event string

const (
	// DatabaseEvent is emitted when the song database has been modified after update.
	DatabaseEvent = Event("database")
	// UpdateEvent is emitted when a database update has started or finished.
	// If the database was modified during the update, the database event is
	// also emitted.
	UpdateEvent = Event("update")
	// StoredPlaylistEvent is emitted when a stored playlist has been modified,
	// renamed, created or deleted.
	StoredPlaylistEvent = Event("stored_playlist")
	// PlaylistEvent is emitted when the current playlist has been modified.
	PlaylistEvent = Event("playlist")
	// PlayerEvent is emitted when the player has been started, stopped or
	// seeked.
	PlayerEvent = Event("player")
	// MixerEvent is emitted when the volume has been changed.
	MixerEvent = Event("mixer")
	// OutputEvent is emitted when an audio output has been added, removed or
	// modified (e.g. renamed, enabled or disabled).
	OutputEvent = Event("output")
	// OptionsEvent is emitted when options like repeat, random, crossfade,
	// replay gain.
	OptionsEvent = Event("options")
	// PartitionEvent is emitted when a partition was added, removed or
	// changed.
	PartitionEvent = Event("partition")
	// StickerEvent is emitted when the sticker database has been modified..
	StickerEvent = Event("sticker")
	// SubscriptionEvent is emitted when a client has subscribed or
	// unsubscribed to a channel.
	SubscriptionEvent = Event("subscription")
	// MessageEvent is emitted when a message was received on a channel this
	// client is subscribed to; this event is only emitted when the queue is
	// empty.
	MessageEvent = Event("message")
)

// Player handles the connection to a single MPD instance.
type Player struct {
	util.Emitter

	clientPool chan *mpd.Client

	network, address string
	passwd           string

	cachedLibrary *cache.Cache
	playlist      player.PlaylistMetaKeeper

	// Sometimes, the volume returned by MPD is invalid, so we have to take
	// care of that ourselves.
	lastVolume int
}

// Connect connects to MPD with an optional username and password.
func Connect(network, address string, mpdPassword *string) (*Player, error) {
	var passwd string
	if mpdPassword != nil {
		passwd = *mpdPassword
	} else {
		passwd = ""
	}

	player := &Player{
		Emitter: util.Emitter{Release: time.Millisecond * 100},
		network: network,
		address: address,
		passwd:  passwd,

		// NOTE: MPD supports up to 10 concurrent connections by default. When
		// this number is reached and ANYTHING tries to connect, the connection
		// rudely closed.
		clientPool: make(chan *mpd.Client, 6),
	}
	player.playlist.Playlist = mpdPlaylist{player: player}
	player.cachedLibrary = cache.NewCache(player)

	// Test the connection.
	client, err := mpd.DialAuthenticated(player.network, player.address, player.passwd)
	if err != nil {
		return nil, err
	}
	client.Close()
	for i := 0; i < cap(player.clientPool); i++ {
		player.clientPool <- nil
	}

	go player.eventLoop()
	go player.mainLoop()
	return player, nil
}

func (pl *Player) withMpd(fn func(*mpd.Client) error) error {
	client := <-pl.clientPool
	if client == nil || client.Ping() != nil {
		var err error
		client, err = mpd.DialAuthenticated(pl.network, pl.address, pl.passwd)
		if err != nil {
			pl.clientPool <- nil
			return fmt.Errorf("error connecting to MPD: %v", err)
		}
	}

	defer func() { pl.clientPool <- client }()
	return fn(client)
}

func (pl *Player) eventLoop() {
	for {
		watcher, err := mpd.NewWatcher(pl.network, pl.address, pl.passwd)
		if err != nil {
			log.Debugf("Could not start watcher: %v", err)
			// Limit the number of reconnection attempts to one per second.
			time.Sleep(time.Second)
			continue
		}
		defer watcher.Close()
		pl.Emit(player.AvailabilityEvent{Available: true})

	loop:
		for {
			select {
			case event := <-watcher.Event:
				pl.Emit(Event(event))
			case <-watcher.Error:
				pl.Emit(player.AvailabilityEvent{Available: false})
				break loop
			}
		}
	}
}

func (pl *Player) mainLoop() {
	listener := pl.Listen()
	defer pl.Unlisten(listener)

	// Helper function to prevent emitting events when an associated value has
	// not changed.
	eventDedup := map[player.Event]interface{}{}
	dedupEmit := func(event player.Event, newValue interface{}) {
		eventName := fmt.Sprintf("%T", event)
		prevValue, ok := eventDedup[eventName]
		eventDedup[eventName] = newValue
		if !ok || !reflect.DeepEqual(prevValue, newValue) {
			pl.Emit(event)
		}
	}

	for event := range listener {
		mpdEvent, ok := event.(Event)
		if !ok {
			continue
		}
		switch mpdEvent {
		case PlayerEvent:
			if state, err := pl.State(); err != nil {
				log.Error(err)
			} else {
				dedupEmit(player.PlayStateEvent{State: state}, state)
			}
			if time, err := pl.Time(); err != nil {
				log.Error(err)
			} else {
				dedupEmit(player.TimeEvent{Time: time}, time)
			}
			fallthrough

		case PlaylistEvent:
			if index, err := pl.TrackIndex(); err != nil {
				log.Error(err)
			} else {
				pl.Emit(player.PlaylistEvent{Index: index})
			}

		case MixerEvent:
			if volume, err := pl.Volume(); err != nil {
				log.Error(err)
			} else {
				dedupEmit(player.VolumeEvent{Volume: volume}, volume)
			}

		case UpdateEvent:
			err := pl.withMpd(func(mpdc *mpd.Client) error {
				status, err := mpdc.Status()
				if err != nil {
					return err
				}
				if _, ok := status["updating_db"]; !ok {
					pl.Emit(library.UpdateEvent{})
				}
				return nil
			})
			if err != nil {
				log.Error(err)
			}
		}
	}
}

// Library implements the player.Player interface.
func (pl *Player) Library() library.Library {
	return pl.cachedLibrary
}

// Tracks implements the library.Library interface.
func (pl *Player) Tracks() ([]library.Track, error) {
	var tracks []library.Track
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		// The MPD listallinfo command breaks for large libraries. So we'll run
		// individual queries for each file in the root to try to get around
		// this weird limitiation.
		filesInRoot, err := mpdc.ListInfo("/")
		if err != nil {
			return fmt.Errorf("error root MPD songs: %v", err)
		}
		var songs []mpd.Attrs
		for _, rootFile := range filesInRoot {
			var filename string
			if f, ok := rootFile["file"]; ok {
				filename = f
			} else if f, ok := rootFile["directory"]; ok {
				filename = f
			}
			ls, err := mpdc.ListAllInfo(filename)
			if err != nil {
				return fmt.Errorf("error getting MPD songs: %v", err)
			}
			songs = append(songs, ls...)
		}

		numDirs := 0
		tracks = make([]library.Track, len(songs))
		for i, song := range songs {
			if _, ok := song["directory"]; ok {
				numDirs++
			} else if err := trackFromMpdSong(mpdc, &song, &tracks[i-numDirs]); err != nil {
				return fmt.Errorf("error mapping MPD song to track: %v", err)
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
		return nil
	})
	return tracks, err
}

// TrackInfo implements the library.Library interface.
func (pl *Player) TrackInfo(identities ...string) ([]library.Track, error) {
	currentTrackURI := ""
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		current, err := mpdc.CurrentSong()
		if err != nil {
			return err
		}
		if file, ok := current["file"]; ok {
			currentTrackURI = mpdToURI(file)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var tracks []library.Track
	err = pl.withMpd(func(mpdc *mpd.Client) error {
		songs := make([]mpd.Attrs, len(identities))
		for i, id := range identities {
			uri := id
			if strings.HasPrefix(uri, uriSchema) {
				s, err := mpdc.ListAllInfo(uriToMpd(uri))
				if err != nil {
					return fmt.Errorf("unable to get info about %v: %v", uri, err)
				}
				if len(s) > 0 {
					songs[i] = s[0]
				}
				continue
			}

			isHTTP := strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "http://")
			if currentTrackURI == uri && isHTTP {
				song, err := mpdc.CurrentSong()
				if err != nil {
					return fmt.Errorf("unable to get info about %v: %v", uri, err)
				}
				songs[i] = song
				songs[i]["Album"] = song["Name"]
			}
		}

		numDirs := 0
		tracks = make([]library.Track, len(songs))
		for i, song := range songs {
			if _, ok := song["directory"]; ok {
				numDirs++
			} else if song != nil {
				if err := trackFromMpdSong(mpdc, &song, &tracks[i-numDirs]); err != nil {
					return err
				}
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
		return nil
	})
	return tracks, err
}

// Lists implements the player.Player interface.
func (pl *Player) Lists() (map[string]player.Playlist, error) {
	playlists := map[string]player.Playlist{}
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		plAttrs, err := mpdc.ListPlaylists()
		if err != nil {
			return err
		}
		for _, attr := range plAttrs {
			playlists[attr["playlist"]] = userPlaylist{
				player: pl,
				name:   attr["playlist"],
			}
		}
		return nil
	})
	return playlists, err
}

// Time implements the player.Player interface.
func (pl *Player) Time() (time.Duration, error) {
	var offset time.Duration
	err := pl.withMpd(func(mpdc *mpd.Client) (err error) {
		status, err := mpdc.Status()
		if err != nil {
			return err
		}
		timef, _ := strconv.ParseFloat(status["elapsed"], 32)
		offset = time.Duration(timef) * time.Second
		return
	})
	return offset, err
}

func (pl *Player) setTimeWith(mpdc *mpd.Client, offset time.Duration) error {
	if offset < 0 {
		return fmt.Errorf("error setting time: negative offset")
	}
	index, err := pl.trackIndexWith(mpdc)
	if err != nil {
		return fmt.Errorf("error getting index for setting time: %v", err)
	}
	if index < 0 {
		return fmt.Errorf("error setting time: negative track index (is any playback happening?)")
	}
	if err := mpdc.Seek(index, int(offset/time.Second)); err != nil {
		return fmt.Errorf("error setting time: %v", err)
	}
	return nil
}

// SetTime implements the player.Player interface.
func (pl *Player) SetTime(offset time.Duration) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		return pl.setTimeWith(mpdc, offset)
	})
}

// SetTrackIndex implements the player.Player interface.
func (pl *Player) SetTrackIndex(trackIndex int) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		if plistLen, err := pl.Playlist().Len(); err != nil {
			return err
		} else if trackIndex >= plistLen {
			return pl.setStateWith(mpdc, player.PlayStateStopped)
		}
		return mpdc.Play(trackIndex)
	})
}

func (pl *Player) trackIndexWith(mpdc *mpd.Client) (int, error) {
	status, err := mpdc.Status()
	if err != nil {
		return -1, err
	}
	cur, ok := statusAttrInt(status, "song")
	if !ok {
		return -1, nil
	}
	return cur, nil
}

// TrackIndex implements the player.Player interface.
func (pl *Player) TrackIndex() (int, error) {
	var trackIndex int
	err := pl.withMpd(func(mpdc *mpd.Client) (err error) {
		trackIndex, err = pl.trackIndexWith(mpdc)
		return
	})
	return trackIndex, err
}

func (pl *Player) stateWith(mpdc *mpd.Client) (player.PlayState, error) {
	status, err := mpdc.Status()
	if err != nil {
		return player.PlayStateInvalid, err
	}

	return map[string]player.PlayState{
		"play":  player.PlayStatePlaying,
		"pause": player.PlayStatePaused,
		"stop":  player.PlayStateStopped,
	}[status["state"]], nil
}

// State implements the player.Player interface.
func (pl *Player) State() (player.PlayState, error) {
	var state player.PlayState
	err := pl.withMpd(func(mpdc *mpd.Client) (err error) {
		state, err = pl.stateWith(mpdc)
		return
	})
	return state, err
}

func (pl *Player) setStateWith(mpdc *mpd.Client, state player.PlayState) error {
	switch state {
	case player.PlayStatePaused:
		return mpdc.Pause(true)
	case player.PlayStatePlaying:
		if plistLen, err := pl.Playlist().Len(); err != nil {
			return fmt.Errorf("error getting playlist length: %v", err)
		} else if plistLen == 0 {
			pl.Emit(player.PlayStateEvent{State: state})
			return nil
		}

		status, err := mpdc.Status()
		if err != nil {
			return fmt.Errorf("error getting status: %v", err)
		}
		if status["state"] == "stop" {
			if err := mpdc.Play(0); err != nil {
				return fmt.Errorf("error starting playback: %v", err)
			}
		} else {
			if err := mpdc.Pause(false); err != nil {
				return fmt.Errorf("error unpausing: %v", err)
			}
		}
	case player.PlayStateStopped:
		if err := mpdc.Stop(); err != nil {
			return fmt.Errorf("error stopping: %v", err)
		}
	default:
		return fmt.Errorf("unknown play state %v", state)
	}
	return nil
}

// SetState implements the player.Player interface.
func (pl *Player) SetState(state player.PlayState) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		return pl.setStateWith(mpdc, state)
	})
}

// Volume implements the player.Player interface.
func (pl *Player) Volume() (int, error) {
	var vol int
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		status, err := mpdc.Status()
		if err != nil {
			return err
		}

		volInt, ok := statusAttrInt(status, "volume")
		if !ok || volInt < 0 {
			// Volume is not present when the playback is stopped.
			vol = pl.lastVolume
			return nil
		}
		vol = volInt
		return nil
	})
	return vol, err
}

// SetVolume implements the player.Player interface.
func (pl *Player) SetVolume(vol int) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		if vol > 100 {
			vol = 100
		} else if vol < 0 {
			vol = 0
		}

		pl.lastVolume = vol
		return mpdc.SetVolume(vol)
	})
}

// Available implements the player.Player interface.
func (pl *Player) Available() bool {
	return pl.withMpd(func(mpdc *mpd.Client) error { return mpdc.Ping() }) == nil
}

// Playlist implements the player.Player interface.
func (pl *Player) Playlist() player.MetaPlaylist {
	return &pl.playlist
}

// TrackArt implements the library.Library interface.
func (pl *Player) TrackArt(track string) (image io.ReadCloser, mime string) {
	pl.withMpd(func(mpdc *mpd.Client) error {
		id := uriToMpd(track)
		numChunks := 0
		if stkNum, err := mpdc.StickerGet(id, "image-nchunks"); err != nil || stkNum == nil {
			return nil
		} else if numChunks, err = strconv.Atoi(stkNum.Value); err != nil {
			return nil
		}

		chunks := make([]io.Reader, 0, numChunks)
		for i := 0; i < numChunks; i++ {
			stkB64Data, err := mpdc.StickerGet(id, fmt.Sprintf("image-%d", i))
			if err != nil || stkB64Data == nil {
				return nil
			}
			chunks = append(chunks, strings.NewReader(stkB64Data.Value))
		}
		image = ioutil.NopCloser(base64.NewDecoder(base64.StdEncoding, io.MultiReader(chunks...)))
		mime = "image/jpeg"
		return nil
	})
	return
}

// Events implements the player.Player interface.
func (pl *Player) Events() *util.Emitter {
	return &pl.Emitter
}

func (pl *Player) String() string {
	return fmt.Sprintf("MPD{%s}", pl.address)
}

type mpdPlaylist struct {
	player *Player
}

func (plist mpdPlaylist) Insert(pos int, tracks ...library.Track) error {
	return plist.player.withMpd(func(mpdc *mpd.Client) error {
		if pos == -1 {
			for _, track := range tracks {
				if _, err := mpdc.AddID(uriToMpd(track.URI), -1); err != nil {
					return fmt.Errorf("error appending %q: %v", track.URI, err)
				}
			}
		} else {
			for i, track := range tracks {
				if _, err := mpdc.AddID(uriToMpd(track.URI), pos+i); err != nil {
					return fmt.Errorf("error inserting %q: %v", track.URI, err)
				}
			}
		}
		return nil
	})
}

func (plist mpdPlaylist) Move(fromPos, toPos int) error {
	return plist.player.withMpd(func(mpdc *mpd.Client) error {
		return mpdc.Move(fromPos, fromPos+1, toPos)
	})
}

func (plist mpdPlaylist) Remove(positions ...int) error {
	return plist.player.withMpd(func(mpdc *mpd.Client) error {
		length, ok := playlistLength(mpdc)
		if !ok {
			return fmt.Errorf("unable to determine playlist length")
		}
		sort.Ints(positions)
		for i := len(positions) - 1; i >= 0; i-- {
			if positions[i] >= length {
				continue
			} else if err := mpdc.Delete(positions[i], positions[i]+1); err != nil {
				return err
			}
		}
		return nil
	})
}

func (plist mpdPlaylist) Tracks() ([]library.Track, error) {
	var tracks []library.Track
	err := plist.player.withMpd(func(mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistInfo(-1, -1)
		if err != nil {
			return err
		}
		tracks = make([]library.Track, len(songs))
		for i, song := range songs {
			if err := trackFromMpdSong(mpdc, &song, &tracks[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tracks, nil
}

func (plist mpdPlaylist) Len() (length int, err error) {
	err = plist.player.withMpd(func(mpdc *mpd.Client) error {
		length, _ = playlistLength(mpdc)
		return nil
	})
	return
}

func playlistLength(mpdc *mpd.Client) (int, bool) {
	status, err := mpdc.Status()
	if err != nil {
		return -1, false
	}
	return statusAttrInt(status, "playlistlength")
}

// Initializes a track from an MPD hash. The hash should be gotten using
// ListAllInfo().
//
// ListAllInfo() and ListInfo() look very much the same but they don't return
// the same thing. Who the fuck thought it was a good idea to mix capitals and
// lowercase?!
func trackFromMpdSong(mpdc *mpd.Client, song *mpd.Attrs, track *library.Track) error {
	if _, ok := (*song)["directory"]; ok {
		return fmt.Errorf("tried to read a directory as local file")
	}

	track.URI = mpdToURI((*song)["file"])
	track.Artist = (*song)["Artist"]
	track.Title = (*song)["Title"]
	track.Genre = (*song)["Genre"]
	track.Album = (*song)["Album"]
	track.AlbumArtist = (*song)["AlbumArtist"]
	track.AlbumDisc = (*song)["Disc"]
	track.AlbumTrack = (*song)["Track"]

	stkNum, _ := mpdc.StickerGet((*song)["file"], "image-nchunks")
	if stkNum != nil {
		_, err := strconv.ParseInt(stkNum.Value, 10, 32)
		track.HasArt = err == nil
	}

	if timeStr := (*song)["Time"]; timeStr != "" {
		duration, err := strconv.ParseInt(timeStr, 10, 32)
		if err != nil {
			return err
		}
		track.Duration = time.Duration(duration) * time.Second
	}

	library.InterpolateMissingFields(track)
	return nil
}

// Helper to get an attribute as an integer from an MPD status.
func statusAttrInt(status mpd.Attrs, attr string) (int, bool) {
	if str, ok := status[attr]; ok {
		if a64, err := strconv.ParseInt(str, 10, 32); err == nil {
			return int(a64), true
		}
	}
	return 0, false
}

func uriToMpd(uri string) string {
	return strings.TrimPrefix(uri, uriSchema)
}

func mpdToURI(song string) string {
	if !strings.Contains(song, "://") {
		return uriSchema + song
	}
	return song
}
