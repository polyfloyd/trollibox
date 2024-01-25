package mpd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fhs/gompd/v2/mpd"

	"trollibox/src/library"
	"trollibox/src/library/cache"
	"trollibox/src/player"
	"trollibox/src/util"
)

const uriSchema = "mpd://"

// Event is an event which signals a change in one of MPD's subsystems.
type mpdEvent string

// nolint:deadcode,varcheck,unused
const (
	// databaseEvent is emitted when the song database has been modified after update.
	databaseEvent = mpdEvent("database")
	// updateEvent is emitted when a database update has started or finished.
	// If the database was modified during the update, the database event is
	// also emitted.
	updateEvent = mpdEvent("update")
	// storedPlaylistEvent is emitted when a stored playlist has been modified,
	// renamed, created or deleted.
	storedPlaylistEvent = mpdEvent("stored_playlist")
	// playlistEvent is emitted when the current playlist has been modified.
	playlistEvent = mpdEvent("playlist")
	// PlayerEvent is emitted when the player has been started, stopped or
	// seeked.
	PlayerEvent = mpdEvent("player")
	// mixerEvent is emitted when the volume has been changed.
	mixerEvent = mpdEvent("mixer")
	// outputEvent is emitted when an audio output has been added, removed or
	// modified (e.g. renamed, enabled or disabled).
	outputEvent = mpdEvent("output")
	// optionsEvent is emitted when options like repeat, random, crossfade,
	// replay gain.
	optionsEvent = mpdEvent("options")
	// partitionEvent is emitted when a partition was added, removed or
	// changed.
	partitionEvent = mpdEvent("partition")
	// stickerEvent is emitted when the sticker database has been modified..
	stickerEvent = mpdEvent("sticker")
	// subscriptionEvent is emitted when a client has subscribed or
	// unsubscribed to a channel.
	subscriptionEvent = mpdEvent("subscription")
	// messageEvent is emitted when a message was received on a channel this
	// client is subscribed to; this event is only emitted when the queue is
	// empty.
	messageEvent = mpdEvent("message")
)

type contextKey int

const clientContextKey = contextKey(1)

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
	lastVolumeLock sync.Mutex
	lastVolume     int
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

func (pl *Player) withMpd(ctx context.Context, fn func(context.Context, *mpd.Client) error) error {
	// Be re-entrant by reusing a previously acquired connection set on the
	// context.
	if client, ok := ctx.Value(clientContextKey).(*mpd.Client); ok {
		return fn(ctx, client)
	}

	// Get a slot from the semaphore.
	var client *mpd.Client
	select {
	case client = <-pl.clientPool:
	case <-ctx.Done():
		return ctx.Err()
	}

	if client == nil || client.Ping() != nil {
		var err error
		client, err = mpd.DialAuthenticated(pl.network, pl.address, pl.passwd)
		if err != nil {
			pl.clientPool <- nil
			return fmt.Errorf("error connecting to MPD: %v / %w", err, player.ErrUnavailable)
		}
	}

	defer func() { pl.clientPool <- client }()
	return fn(context.WithValue(ctx, clientContextKey, client), client)
}

func (pl *Player) eventLoop() {
	for {
		watcher, err := mpd.NewWatcher(pl.network, pl.address, pl.passwd)
		if err != nil {
			slog.Debug("Could not start watcher", "error", err)
			// Limit the number of reconnection attempts to one per second.
			time.Sleep(time.Second)
			continue
		}
		defer watcher.Close()

	loop:
		for {
			select {
			case event := <-watcher.Event:
				pl.Emit(mpdEvent(event))
			case <-watcher.Error:
				break loop
			}
		}
	}
}

func (pl *Player) mainLoop() {
	ctx := context.Background()
	listener := pl.Listen(ctx)

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
		mpdEvent, ok := event.(mpdEvent)
		if !ok {
			continue
		}
		switch mpdEvent {
		case PlayerEvent:
			status, err := pl.Status(ctx)
			if err != nil {
				slog.Error("Could not get MPD status", "error", err)
				continue
			}
			dedupEmit(player.PlayStateEvent{State: status.PlayState}, status.PlayState)
			dedupEmit(player.TimeEvent{Time: status.Time}, status.Time)
			fallthrough

		case playlistEvent:
			status, err := pl.Status(ctx)
			if err != nil {
				slog.Error("Could not get MPD status", "error", err)
				continue
			}
			pl.Emit(player.PlaylistEvent{TrackIndex: status.TrackIndex})

		case mixerEvent:
			status, err := pl.Status(ctx)
			if err != nil {
				slog.Error("Could not get MPD status", "error", err)
				continue
			}
			dedupEmit(player.VolumeEvent{Volume: status.Volume}, status.Volume)

		case updateEvent:
			err := pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
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
				slog.Error("Could not get MPD status", "error", err)
			}
		}
	}
}

// Library implements the player.Player interface.
func (pl *Player) Library() library.Library {
	return pl.cachedLibrary
}

// Tracks implements the library.Library interface.
func (pl *Player) Tracks(ctx context.Context) (tracks []library.Track, err error) {
	err = pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) (err error) {
		// The MPD listallinfo command breaks for large libraries. So we'll run
		// individual queries for each file in the root to try to get around
		// this weird limitiation.
		tracks, err = tracksInDirectory(ctx, mpdc, "/")
		return
	})
	return
}

// TrackInfo implements the library.Library interface.
func (pl *Player) TrackInfo(ctx context.Context, identities ...string) (tracks []library.Track, err error) {
	err = pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		current, err := mpdc.CurrentSong()
		if err != nil {
			return err
		}
		currentTrackURI := ""
		if file, ok := current["file"]; ok {
			currentTrackURI = mpdToURI(file)
		}

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
				if err := trackFromMpdSong(mpdc, song, &tracks[i-numDirs]); err != nil {
					return err
				}
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
		return nil
	})
	return
}

// Lists implements the player.Player interface.
func (pl *Player) Lists(ctx context.Context) (playlists map[string]player.Playlist[library.Track], err error) {
	err = pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		plAttrs, err := mpdc.ListPlaylists()
		if err != nil {
			return err
		}
		playlists = map[string]player.Playlist[library.Track]{}
		for _, attr := range plAttrs {
			playlists[attr["playlist"]] = userPlaylist{
				player: pl,
				name:   attr["playlist"],
			}
		}
		return nil
	})
	return
}

// Status implements the player.Player interface.
func (pl *Player) Status(ctx context.Context) (status *player.Status, err error) {
	err = pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) (err error) {
		mpdStatus, err := mpdc.Status()
		if err != nil {
			return err
		}
		timef, _ := strconv.ParseFloat(mpdStatus["elapsed"], 32)

		trackIndex, _ := statusAttrInt(mpdStatus, "song")

		playState := map[string]player.PlayState{
			"play":  player.PlayStatePlaying,
			"pause": player.PlayStatePaused,
			"stop":  player.PlayStateStopped,
		}[mpdStatus["state"]]

		volume, ok := statusAttrInt(mpdStatus, "volume")
		if !ok || volume < 0 {
			// Volume is not present when the playback is stopped.
			pl.lastVolumeLock.Lock()
			defer pl.lastVolumeLock.Unlock()
			volume = pl.lastVolume
		}

		status = &player.Status{
			TrackIndex: trackIndex,
			Time:       time.Duration(timef) * time.Second,
			PlayState:  playState,
			Volume:     volume,
		}
		return
	})
	return
}

// SetTime implements the player.Player interface.
func (pl *Player) SetTime(ctx context.Context, offset time.Duration) error {
	return pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		if offset < 0 {
			return fmt.Errorf("error setting time: negative offset")
		}
		status, err := pl.Status(ctx)
		if err != nil {
			return fmt.Errorf("error getting index for setting time: %v", err)
		}
		if status.TrackIndex < 0 {
			return fmt.Errorf("error setting time: negative track index (is any playback happening?)")
		}
		if err := mpdc.Seek(status.TrackIndex, int(offset/time.Second)); err != nil {
			return fmt.Errorf("error setting time: %v", err)
		}
		return nil
	})
}

// SetTrackIndex implements the player.Player interface.
func (pl *Player) SetTrackIndex(ctx context.Context, trackIndex int) error {
	return pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		if plistLen, err := pl.Playlist().Len(ctx); err != nil {
			return err
		} else if trackIndex >= plistLen {
			return pl.SetState(ctx, player.PlayStateStopped)
		}
		return mpdc.Play(trackIndex)
	})
}

// SetState implements the player.Player interface.
func (pl *Player) SetState(ctx context.Context, state player.PlayState) error {
	err := pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		switch state {
		case player.PlayStatePaused:
			return mpdc.Pause(true)
		case player.PlayStatePlaying:
			if plistLen, err := pl.Playlist().Len(ctx); err != nil {
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
			return fmt.Errorf("unknown play state %q", state)
		}
		return nil
	})
	return err
}

// SetVolume implements the player.Player interface.
func (pl *Player) SetVolume(ctx context.Context, vol int) error {
	return pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		if vol > 100 {
			vol = 100
		} else if vol < 0 {
			vol = 0
		}

		pl.lastVolumeLock.Lock()
		defer pl.lastVolumeLock.Unlock()
		pl.lastVolume = vol
		return mpdc.SetVolume(vol)
	})
}

// Playlist implements the player.Player interface.
func (pl *Player) Playlist() player.Playlist[player.MetaTrack] {
	return &pl.playlist
}

// TrackArt implements the library.Library interface.
func (pl *Player) TrackArt(ctx context.Context, track string) (art *library.Art, err error) {
	err = pl.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		tt, err := pl.TrackInfo(ctx, track)
		if err != nil {
			return err
		}

		imageData, err := mpdc.ReadPicture(uriToMpd(track))
		if err != nil {
			if err.Error() == "no binary data found in response" {
				return library.ErrNoArt
			}
			return err
		}
		art = &library.Art{
			ImageData: imageData,
			MimeType:  http.DetectContentType(imageData),
			ModTime:   tt[0].ModTime,
		}
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

func (plist mpdPlaylist) Insert(ctx context.Context, pos int, tracks ...library.Track) error {
	return plist.player.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		length, ok := playlistLength(mpdc)
		if !ok {
			return fmt.Errorf("unable to determine playlist length")
		}
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
		if length == 0 {
			if err := mpdc.Play(0); err != nil {
				return err
			}
			// Play the 0th track in the playlist if there were no tracks in the playlist before queing the requested track(s)
			// otherwise the track(s) will be queued before a random autoplayer track
		}
		return nil
	})
}

func (plist mpdPlaylist) Move(ctx context.Context, fromPos, toPos int) error {
	return plist.player.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		return mpdc.Move(fromPos, fromPos+1, toPos)
	})
}

func (plist mpdPlaylist) Remove(ctx context.Context, positions ...int) error {
	return plist.player.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
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

func (plist mpdPlaylist) Tracks(ctx context.Context) (tracks []library.Track, err error) {
	err = plist.player.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistInfo(-1, -1)
		if err != nil {
			return err
		}
		tracks = make([]library.Track, len(songs))
		for i, song := range songs {
			if err := trackFromMpdSong(mpdc, song, &tracks[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return
}

func (plist mpdPlaylist) Len(ctx context.Context) (length int, err error) {
	err = plist.player.withMpd(ctx, func(ctx context.Context, mpdc *mpd.Client) error {
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

func tracksInDirectory(ctx context.Context, mpdc *mpd.Client, path string) ([]library.Track, error) {
	info, err := mpdc.ListInfo(path)
	if err != nil {
		return nil, fmt.Errorf("mpd list info %q: %v", path, err)
	}

	tracks := make([]library.Track, 0, len(info))
	for _, info := range info {
		if _, ok := info["file"]; ok {
			var track library.Track
			if err := trackFromMpdSong(mpdc, info, &track); err != nil {
				return nil, fmt.Errorf("error mapping MPD song to track: %v", err)
			}
			tracks = append(tracks, track)
		} else if dir, ok := info["directory"]; ok {
			tt, err := tracksInDirectory(ctx, mpdc, dir)
			if err != nil {
				return nil, err
			}
			tracks = append(tracks, tt...)
		}
		// TODO: key "playlist"
	}
	return tracks, nil
}

// trackFromMpdSong initializes a track from an MPD hash. The hash should be gotten using
// ListAllInfo().
func trackFromMpdSong(mpdc *mpd.Client, song mpd.Attrs, track *library.Track) error {
	// ListAllInfo() and ListInfo() look very much the same but they don't return the same thing.
	// Why capitals and lowercase are mixed is beyond me.
	for k, v := range song {
		song[strings.ToLower(k)] = v
	}

	if _, ok := song["directory"]; ok {
		return fmt.Errorf("tried to read a directory as local file")
	}

	track.URI = mpdToURI(song["file"])
	track.Artist = song["artist"]
	track.Title = song["title"]
	track.Genre = song["genre"]
	track.Album = song["album"]
	track.AlbumArtist = song["albumartist"]
	track.AlbumDisc = song["disc"]
	track.AlbumTrack = song["track"]
	modTime, err := time.Parse(time.RFC3339, song["last-modified"])
	if err != nil {
		slog.Warn("Could not parse song time", "error", err, "song", song)
	} else {
		track.ModTime = modTime
	}

	if timeStr := song["time"]; timeStr != "" {
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
