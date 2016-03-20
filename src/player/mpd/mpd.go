package mpd

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	player "../"
	"../../util"
	"github.com/polyfloyd/gompd/mpd"
)

const URI_SCHEMA = "mpd://"

type Player struct {
	util.Emitter

	clientPool chan *mpd.Client

	network, address string
	passwd           string

	playlist player.PlaylistMetaKeeper

	// Sometimes, the volume returned by MPD is invalid, so we have to take
	// care of that ourselves.
	lastVolume float32
}

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
			return err
		}
	}

	defer func() { pl.clientPool <- client }()
	return fn(client)
}

func (pl *Player) eventLoop() {
	for {
		watcher, err := mpd.NewWatcher(pl.network, pl.address, pl.passwd)
		if err != nil {
			// Limit the number of reconnection attempts to one per second.
			time.Sleep(time.Second)
			continue
		}
		defer watcher.Close()
		pl.Emit("availability")

	loop:
		for {
			select {
			case event := <-watcher.Event:
				pl.Emit("mpd-" + event)
			case <-watcher.Error:
				pl.Emit("availability")
				break loop
			}
		}
	}
}

func (pl *Player) mainLoop() {
	listener := pl.Listen()
	defer pl.Unlisten(listener)

	for {
		switch event := <-listener; event {
		case "mpd-player":
			pl.Emit("playstate")
			pl.Emit("time")
			fallthrough

		case "mpd-playlist":
			pl.Emit("playlist")

		case "mpd-mixer":
			pl.Emit("volume")

		case "mpd-update":
			err := pl.withMpd(func(mpdc *mpd.Client) error {
				status, err := mpdc.Status()
				if err != nil {
					return err
				}
				if _, ok := status["updating_db"]; !ok {
					pl.Emit("tracks")
				}
				return nil
			})
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (pl *Player) Tracks() ([]player.Track, error) {
	var tracks []player.Track
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		songs, err := mpdc.ListAllInfo("/")
		if err != nil {
			return err
		}

		numDirs := 0
		tracks = make([]player.Track, len(songs))
		for i, song := range songs {
			if _, ok := song["directory"]; ok {
				numDirs++
			} else if err := trackFromMpdSong(mpdc, &song, &tracks[i-numDirs]); err != nil {
				return err
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
		return nil
	})
	return tracks, err
}

func (pl *Player) TrackInfo(identities ...string) ([]player.Track, error) {
	currentTrackUri := ""
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		current, err := mpdc.CurrentSong()
		if err != nil {
			return err
		}
		if file, ok := current["file"]; ok {
			currentTrackUri = mpdToUri(file)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var tracks []player.Track
	err = pl.withMpd(func(mpdc *mpd.Client) error {
		songs := make([]mpd.Attrs, len(identities))
		for i, id := range identities {
			uri := id
			if strings.HasPrefix(uri, URI_SCHEMA) {
				s, err := mpdc.ListAllInfo(uriToMpd(uri))
				if err != nil {
					return fmt.Errorf("Unable to get info about %v: %v", uri, err)
				}
				if len(s) > 0 {
					songs[i] = s[0]
				}
			} else if ok, _ := regexp.MatchString("https?:\\/\\/", uri); ok && currentTrackUri == uri {
				song, err := mpdc.CurrentSong()
				if err != nil {
					return fmt.Errorf("Unable to get info about %v: %v", uri, err)
				}
				songs[i] = song
				songs[i]["Album"] = song["Name"]
			}
		}

		numDirs := 0
		tracks = make([]player.Track, len(songs))
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
	index, err := pl.trackIndexWith(mpdc)
	if err != nil {
		return err
	}
	return mpdc.Seek(index, int(offset/time.Second))
}

func (pl *Player) SetTime(offset time.Duration) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		return pl.setTimeWith(mpdc, offset)
	})
}

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
		status, err := mpdc.Status()
		if err != nil {
			return err
		}
		if status["state"] == "stop" {
			return mpdc.Play(0)
		} else {
			return mpdc.Pause(false)
		}
	case player.PlayStateStopped:
		return mpdc.Stop()
	default:
		return fmt.Errorf("Unknown play state %v", state)
	}
}

func (pl *Player) SetState(state player.PlayState) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		return pl.setStateWith(mpdc, state)
	})
}

func (pl *Player) Volume() (float32, error) {
	var vol float32
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		status, err := mpdc.Status()
		if err != nil {
			return err
		}

		volInt, ok := statusAttrInt(status, "volume")
		if !ok {
			// Volume should always be present.
			return fmt.Errorf("No volume property is present in the MPD status")
		}

		vol = float32(volInt) / 100
		if vol < 0 {
			// Happens sometimes when nothing is playing.
			vol = pl.lastVolume
		}
		return nil
	})
	return vol, err
}

func (pl *Player) SetVolume(vol float32) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		if vol > 1 {
			vol = 1
		} else if vol < 0 {
			vol = 0
		}

		pl.lastVolume = vol
		return mpdc.SetVolume(int(vol * 100))
	})
}

func (pl *Player) Available() bool {
	return pl.withMpd(func(mpdc *mpd.Client) error { return mpdc.Ping() }) == nil
}

func (pl *Player) Playlist() player.MetaPlaylist {
	return &pl.playlist
}

func (pl *Player) TrackArt(track string) (image io.ReadCloser, mime string) {
	pl.withMpd(func(mpdc *mpd.Client) error {
		id := uriToMpd(track)
		numChunks := 0
		if strNum, err := mpdc.StickerGet(id, "image-nchunks"); err == nil {
			if num, err := strconv.ParseInt(strNum, 10, 32); err == nil {
				numChunks = int(num)
			}
		}
		if numChunks == 0 {
			return nil
		}

		chunks := make([]io.Reader, numChunks+1)
		totalLength := 0
		for i := 0; i < numChunks; i++ {
			if b64Data, err := mpdc.StickerGet(id, fmt.Sprintf("image-%v", i)); err != nil {
				return nil
			} else {
				chunks[i] = strings.NewReader(b64Data)
				totalLength += len(b64Data)
			}
		}
		// The padding seems to be getting lost somewhere along the way from MPD to here.
		chunks[len(chunks)-1] = strings.NewReader([]string{"", "=", "==", "==="}[totalLength%4])
		image = ioutil.NopCloser(base64.NewDecoder(base64.StdEncoding, io.MultiReader(chunks...)))
		mime = "image/jpeg"
		return nil
	})
	return
}

func (pl *Player) Events() *util.Emitter {
	return &pl.Emitter
}

func (pl *Player) String() string {
	return fmt.Sprintf("MPD{%s}", pl.address)
}

type mpdPlaylist struct {
	player *Player
}

func (plist mpdPlaylist) Insert(pos int, tracks ...player.Track) error {
	return plist.player.withMpd(func(mpdc *mpd.Client) error {
		if pos == -1 {
			for _, track := range tracks {
				if _, err := mpdc.AddID(uriToMpd(track.Uri), -1); err != nil {
					return err
				}
			}
		} else {
			for i, track := range tracks {
				if _, err := mpdc.AddID(uriToMpd(track.Uri), pos+i); err != nil {
					return err
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
			return fmt.Errorf("Unable to determine playlist length")
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

func (plist mpdPlaylist) Tracks() ([]player.Track, error) {
	var tracks []player.Track
	err := plist.player.withMpd(func(mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistInfo(-1, -1)
		if err != nil {
			return err
		}
		tracks = make([]player.Track, len(songs))
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

func (plist mpdPlaylist) Len() (int, error) {
	tracks, err := plist.Tracks()
	if err != nil {
		return -1, err
	}
	return len(tracks), err
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
func trackFromMpdSong(mpdc *mpd.Client, song *mpd.Attrs, track *player.Track) error {
	if _, ok := (*song)["directory"]; ok {
		return fmt.Errorf("Tried to read a directory as local file")
	}

	track.Uri = mpdToUri((*song)["file"])
	track.Artist = (*song)["Artist"]
	track.Title = (*song)["Title"]
	track.Genre = (*song)["Genre"]
	track.Album = (*song)["Album"]
	track.AlbumArtist = (*song)["AlbumArtist"]
	track.AlbumDisc = (*song)["Disc"]
	track.AlbumTrack = (*song)["Track"]

	strNum, _ := mpdc.StickerGet((*song)["file"], "image-nchunks")
	_, err := strconv.ParseInt(strNum, 10, 32)
	track.HasArt = err == nil

	if timeStr := (*song)["Time"]; timeStr != "" {
		if duration, err := strconv.ParseInt(timeStr, 10, 32); err != nil {
			return err
		} else {
			track.Duration = time.Duration(duration) * time.Second
		}
	}

	player.InterpolateMissingFields(track)
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
	return strings.TrimPrefix(uri, URI_SCHEMA)
}

func mpdToUri(song string) string {
	if strings.Index(song, "://") == -1 {
		return URI_SCHEMA + song
	}
	return song
}
