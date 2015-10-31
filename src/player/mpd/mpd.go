package mpd

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"

	player "../"
	"../event"
	"github.com/polyfloyd/gompd/mpd"
)

var nonMpdTrackUri = regexp.MustCompile("^[a-z]+:\\/\\/")

type Track struct {
	player *Player

	uri         string
	artist      string
	title       string
	genre       string
	album       string
	albumArtist string
	albumTrack  string
	albumDisc   string
	duration    int
}

func (track Track) Uri() string         { return track.uri }
func (track Track) Artist() string      { return track.artist }
func (track Track) Title() string       { return track.title }
func (track Track) Genre() string       { return track.genre }
func (track Track) Album() string       { return track.album }
func (track Track) AlbumArtist() string { return track.albumArtist }
func (track Track) AlbumTrack() string  { return track.albumTrack }
func (track Track) AlbumDisc() string   { return track.albumDisc }
func (track Track) Duration() int       { return track.duration }

func (track Track) Art() (image io.ReadCloser, mime string) {
	track.player.withMpd(func(mpdc *mpd.Client) error {
		numChunks := 0
		if strNum, err := mpdc.StickerGet(track.uri, "image-nchunks"); err == nil {
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
			if b64Data, err := mpdc.StickerGet(track.uri, fmt.Sprintf("image-%v", i)); err != nil {
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

type PlaylistTrack struct {
	Track
	playlistAttrs
}

func (plTrack PlaylistTrack) Progress() int {
	return plTrack.progress
}

func (plTrack PlaylistTrack) QueuedBy() string {
	return plTrack.queuedBy
}

type playlistAttrs struct {
	progress int
	queuedBy string
}

type Player struct {
	*event.Emitter

	// Running the idle routine on the same connection as the main connection
	// will fuck things up badly.
	mpdWatcher *mpd.Watcher

	addr, passwd string

	// A map containing properties related to tracks currently in the queue.
	playlistAttrs map[string]playlistAttrs

	// Sometimes, the volume returned by MDP is invalid, so we have to take
	// care of that ourselves.
	lastVolume float32

	// We use this value to determine wether the currently playing track has
	// changed.
	lastTrack string
}

func NewPlayer(mpdHost string, mpdPort int, mpdPassword *string) (*Player, error) {
	addr := fmt.Sprintf("%v:%v", mpdHost, mpdPort)

	var passwd string
	if mpdPassword != nil {
		passwd = *mpdPassword
	} else {
		passwd = ""
	}

	clientWatcher, err := mpd.NewWatcher("tcp", addr, passwd)
	if err != nil {
		return nil, err
	}

	player := &Player{
		Emitter:       event.NewEmitter(),
		mpdWatcher:    clientWatcher,
		playlistAttrs: map[string]playlistAttrs{},

		addr:   addr,
		passwd: passwd,
	}

	go player.eventLoop()
	go player.playlistLoop()

	return player, nil
}

func (pl *Player) withMpd(fn func(*mpd.Client) error) error {
	client, err := mpd.DialAuthenticated("tcp", pl.addr, pl.passwd)
	if err != nil {
		return err
	}
	defer client.Close()
	return fn(client)
}

func (pl *Player) eventLoop() {
	for {
		select {
		case event := <-pl.mpdWatcher.Event:
			pl.Emit(event)
		case err := <-pl.mpdWatcher.Error:
			log.Println(err)
		}
	}
}

func (pl *Player) playlistLoop() {
	listener := pl.Listen()
	defer pl.Unlisten(listener)
	go func() { // Bootstrap the cycle
		listener <- "playlist"
		listener <- "player"
	}()
	for {
		switch <-listener {
		case "player":
			err := pl.withMpd(func(mpdc *mpd.Client) error {
				if err := pl.removePlayedTracks(mpdc); err != nil {
					return err
				}

				// Emit an event to indicate that the playlist has ended.
				if playlist, err := pl.Playlist(); err != nil {
					return err
				} else if len(playlist) == 0 {
					pl.Emit("playlist-end")
				}
				return nil
			})
			if err != nil {
				log.Println(err)
			}

		case "playlist":
			err := pl.withMpd(func(mpdc *mpd.Client) error {
				if err := pl.recordPlayCount(mpdc); err != nil {
					return err
				}
				return pl.updatePlaylistAttrs(mpdc)
			})
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (pl *Player) removePlayedTracks(mpdc *mpd.Client) error {
	status, err := mpdc.Status()
	if err != nil {
		return err
	}
	songIndex := 0
	if str, ok := status["song"]; ok {
		if song64, err := strconv.ParseInt(str, 10, 32); err != nil {
			return err
		} else {
			songIndex = int(song64)
		}
	} else if status["state"] == "stop" && status["playlistlength"] != "0" {
		// Fix to make sure the previous track is not played twice.
		// TODO: Some funny stuff happens when MPD receives a stop command.
		songIndex = 1
	}
	if songIndex != 0 {
		if err := mpdc.Delete(0, songIndex); err != nil {
			return err
		}
	}
	return nil
}

func (pl *Player) recordPlayCount(mpdc *mpd.Client) error {
	songs, err := mpdc.PlaylistInfo(-1, -1)
	if err != nil {
		return err
	}
	if len(songs) == 0 {
		return nil
	}

	// TODO: If one track is followed by another track with the same
	// ID, the next block will not be executed, leaving the playcount
	// unchanged.
	currentUri := songs[0]["file"]
	if pl.lastTrack != currentUri {
		// Streams can't have stickers.
		if isMpdUri(currentUri) {
			// Increment the playcount for pl track.
			var playCount int64
			if str, err := mpdc.StickerGet(currentUri, "play-count"); err == nil {
				playCount, _ = strconv.ParseInt(str, 10, 32)
			}
			if err := mpdc.StickerSet(currentUri, "play-count", strconv.FormatInt(playCount+1, 10)); err != nil {
				return fmt.Errorf("Unable to set play-count %v", err)
			}
		}

		pl.lastTrack = currentUri
	}
	return nil
}

func (pl *Player) updatePlaylistAttrs(mpdc *mpd.Client) error {
	songs, err := mpdc.PlaylistInfo(-1, -1)
	if err != nil {
		return err
	}

	// Synchronize queue attributes.
	// Remove tracks that are no longer in the list
trackRemoveLoop:
	for id := range pl.playlistAttrs {
		for _, song := range songs {
			if song["file"] == id {
				continue trackRemoveLoop
			}
		}
		delete(pl.playlistAttrs, id)
	}

	// Initialize tracks that were wiped due to restarts or not added using
	// Trollibox.
trackInitLoop:
	for _, song := range songs {
		for id := range pl.playlistAttrs {
			if song["file"] == id {
				continue trackInitLoop
			}
		}
		pl.playlistAttrs[song["file"]] = playlistAttrs{
			// Assume the track was queued by a human.
			queuedBy: "user",
			progress: 0,
		}
	}
	return nil
}

func (pl *Player) trackFromMpdSong(song *mpd.Attrs, track *Track, mpdc *mpd.Client) {
	track.player = pl

	if _, ok := (*song)["directory"]; ok {
		panic("Tried to read a directory as local file")
	}

	track.uri = (*song)["file"]
	track.artist = (*song)["Artist"]
	track.title = (*song)["Title"]
	track.genre = (*song)["Genre"]
	track.album = (*song)["Album"]
	track.albumArtist = (*song)["AlbumArtist"]
	track.albumDisc = (*song)["Disc"]
	track.albumTrack = (*song)["Track"]

	// Who the fuck thought it was a good idea to mix capitals and lowercase
	// for the time?!
	var timeStr string
	if str, ok := (*song)["Time"]; ok {
		timeStr = str
	} else if str, ok := (*song)["time"]; ok {
		timeStr = str
	}

	if timeStr != "" {
		if duration, err := strconv.ParseInt(timeStr, 10, 32); err != nil {
			panic(err)
		} else {
			track.duration = int(duration)
		}
	}
}

func (pl *Player) playlistTrackFromMpdSong(song *mpd.Attrs, plTrack *PlaylistTrack, mpdc *mpd.Client) {
	pl.trackFromMpdSong(song, &plTrack.Track, mpdc)
	plTrack.playlistAttrs = pl.playlistAttrs[plTrack.Track.Uri()]
}

func (pl *Player) Queue(track player.TrackIdentity, queuedBy string) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		pl.playlistAttrs[track.Uri()] = playlistAttrs{
			queuedBy: queuedBy,
			progress: 0,
		}
		return mpdc.Add(track.Uri())
	})
}

func (pl *Player) Volume() (float32, error) {
	var vol float32
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		var status mpd.Attrs
		status, err := mpdc.Status()
		if err != nil {
			return err
		}

		volStr, ok := status["volume"]
		if !ok {
			// Volume should always be present
			return fmt.Errorf("No volume property is present in the MPD status")
		}

		rawVol, err := strconv.ParseInt(volStr, 10, 32)
		if err != nil {
			return err
		}

		vol = float32(rawVol) / 100
		// Happens sometimes when nothing is playing
		if vol < 0 {
			vol = pl.lastVolume
		}
		return nil
	})
	return vol, err
}

func (player *Player) SetVolume(vol float32) error {
	return player.withMpd(func(mpdc *mpd.Client) error {
		if vol > 1 {
			vol = 1
		} else if vol < 0 {
			vol = 0
		}

		player.lastVolume = vol
		return mpdc.SetVolume(int(vol * 100))
	})
}

func (pl *Player) ListTracks(path string, recursive bool) ([]player.Track, error) {
	var tracks []player.Track
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		if path == "" {
			path = "/"
		}

		var err error
		var songs []mpd.Attrs
		if recursive {
			songs, err = mpdc.ListAllInfo(path)
		} else {
			songs, err = mpdc.ListInfo(path)
		}
		if err != nil {
			return err
		}

		numDirs := 0
		tracks = make([]player.Track, len(songs))
		for i, song := range songs {
			track := &Track{}
			if _, ok := song["directory"]; ok {
				numDirs++
			} else {
				pl.trackFromMpdSong(&song, track, mpdc)
				tracks[i-numDirs] = track
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
		return nil
	})
	return tracks, err
}

func (pl *Player) Playlist() ([]player.PlaylistTrack, error) {
	var tracks []player.PlaylistTrack
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistInfo(-1, -1)
		if err != nil {
			return err
		}

		tracks = make([]player.PlaylistTrack, len(songs))
		for i, song := range songs {
			plTrack := &PlaylistTrack{}
			pl.playlistTrackFromMpdSong(&song, plTrack, mpdc)
			tracks[i] = plTrack
		}

		if len(tracks) > 0 {
			status, err := mpdc.Status()
			if err != nil {
				return err
			}

			var progressf float64
			progressf, err = strconv.ParseFloat(status["elapsed"], 32)
			if err != nil {
				progressf = 0
			}
			tracks[0].(*PlaylistTrack).progress = int(progressf)
		}
		return nil
	})
	return tracks, err
}

func (pl *Player) SetPlaylist(tracks []player.TrackIdentity) error {
	playlist, err := pl.Playlist()
	if err != nil {
		return err
	}

	return pl.withMpd(func(mpd *mpd.Client) error {
		// Playing track is not the first track of the new list? Remove it so we
		// can overwrite it.
		delStart := 0
		if len(playlist) > 0 && playlist[0].Uri() == tracks[0].Uri() {
			// Don't queue the first track twice.
			delStart = 1
			tracks = tracks[1:]
		}

		// Clear the playlist. Maybe a bit slow when the playlist is really large.
		if delStart != len(playlist) {
			if err := mpd.Delete(delStart, len(playlist)); err != nil {
				return err
			}
		}

		// Queue the new tracks.
		cmd := mpd.BeginCommandList()
		for _, track := range tracks {
			cmd.Add(track.Uri())
		}
		return cmd.End()
	})
}

func (player *Player) Seek(progress int) error {
	return player.withMpd(func(mpdc *mpd.Client) error {
		status, err := mpdc.Status()
		if err != nil {
			return err
		}

		if str, ok := status["songid"]; !ok {
			// No track is currently being played.
			return nil
		} else if id, perr := strconv.ParseInt(str, 10, 32); err != nil {
			return perr
		} else {
			return mpdc.SeekID(int(id), progress)
		}
	})
}

func (player *Player) Next() error {
	return player.withMpd(func(mpdc *mpd.Client) error {
		if err := mpdc.Next(); err != nil {
			return err
		}
		return mpdc.Delete(0, 1)
	})
}

func (pl *Player) State() (player.PlayState, error) {
	var state player.PlayState
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		status, err := mpdc.Status()
		if err != nil {
			return err
		}

		state = map[string]player.PlayState{
			"play":  player.PlayStatePlaying,
			"pause": player.PlayStatePaused,
			"stop":  player.PlayStateStopped,
		}[status["state"]]
		return nil
	})
	return state, err
}

func (pl *Player) SetState(state player.PlayState) error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
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
	})
}

func (player *Player) Events() *event.Emitter {
	return player.Emitter
}

func isMpdUri(uri string) bool {
	if len(uri) == 0 {
		return false
	}
	return !nonMpdTrackUri.MatchString(uri)
}
