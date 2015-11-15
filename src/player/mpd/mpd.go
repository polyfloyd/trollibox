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
	"sync"
	"time"

	player "../"
	"../../util"
	"github.com/polyfloyd/gompd/mpd"
)

const URI_SCHEMA = "mpd://"

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
	duration    time.Duration
}

func (track Track) Uri() string             { return mpdToUri(track.uri) }
func (track Track) Artist() string          { return track.artist }
func (track Track) Title() string           { return track.title }
func (track Track) Genre() string           { return track.genre }
func (track Track) Album() string           { return track.album }
func (track Track) AlbumArtist() string     { return track.albumArtist }
func (track Track) AlbumTrack() string      { return track.albumTrack }
func (track Track) AlbumDisc() string       { return track.albumDisc }
func (track Track) Duration() time.Duration { return track.duration }

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

type playlistAttrs struct {
	progress int
	queuedBy string
}

type Player struct {
	*util.Emitter

	addr, passwd string

	playlist     []player.PlaylistTrack
	playlistLock sync.Mutex

	// Sometimes, the volume returned by MPD is invalid, so we have to take
	// care of that ourselves.
	lastVolume float32

	// We use this value to determine wether the currently playing track has
	// changed so its play count can be updated.
	lastTrack string

	playlistWasSet bool
}

func NewPlayer(mpdHost string, mpdPort int, mpdPassword *string) (*Player, error) {
	addr := fmt.Sprintf("%v:%v", mpdHost, mpdPort)

	var passwd string
	if mpdPassword != nil {
		passwd = *mpdPassword
	} else {
		passwd = ""
	}

	player := &Player{
		Emitter: util.NewEmitter(),
		addr:    addr,
		passwd:  passwd,
	}

	// Test the connecting to MPD.
	if err := player.withMpd(func(_ *mpd.Client) error { return nil }); err != nil {
		return nil, err
	}

	go player.eventLoop()
	go player.mainLoop()
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
		watcher, err := mpd.NewWatcher("tcp", pl.addr, pl.passwd)
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
	go func() { // Bootstrap the cycle
		listener <- "mpd-playlist"
	}()
	for {
		switch <-listener {
		case "mpd-player":
			pl.Emit("playstate")
			fallthrough

		case "mpd-playlist":
			err := pl.withMpd(func(mpdc *mpd.Client) error {
				pl.playlistLock.Lock()
				defer pl.playlistLock.Unlock()

				if err := pl.removePlayedTracks(mpdc); err != nil {
					return err
				}
				if changed, err := pl.reloadPlaylist(mpdc); err != nil {
					return err
				} else if !changed && !pl.playlistWasSet {
					return nil
				}
				pl.playlistWasSet = false

				pl.Emit("playlist")
				if len(pl.playlist) == 0 {
					pl.Emit("playlist-end")
					return nil
				}

				cur := pl.playlist[0]
				if pl.lastTrack != "" && pl.lastTrack != cur.Uri() {
					// TODO: If one track is followed by another track with the same
					// ID, this block will not be executed, leaving the playcount
					// unchanged.

					// Seek using the progress attr when the track starts playing.
					if cur.Progress != 0 {
						if err := pl.Seek(cur.Progress); err != nil {
							return err
						}
					}

					if err := incrementPlayCount(cur.Uri(), mpdc); err != nil {
						return err
					}
				}
				pl.lastTrack = cur.Uri()

				return nil
			})
			if err != nil {
				log.Println(err)
			}

		case "mpd-mixer":
			pl.Emit("volume")

		case "mpd-update":
			pl.Emit("tracks")
		}
	}
}

func (pl *Player) removePlayedTracks(mpdc *mpd.Client) error {
	status, err := mpdc.Status()
	if err != nil {
		return err
	}

	if songIndex, _ := statusAttrInt(status, "song"); songIndex > 0 {
		return mpdc.Delete(0, songIndex)
	}
	return nil
}

// Synchronizes the MPD server's playlist with the playlist retained by trollibox.
func (pl *Player) reloadPlaylist(mpdc *mpd.Client) (bool, error) {
	songs, err := mpdc.PlaylistInfo(-1, -1)
	if err != nil {
		return false, err
	}
	uris := make([]string, len(songs))
	for i, song := range songs {
		uris[i] = mpdToUri(song["file"])
	}

	// Check wether the argument playlist is equal to the stored playlist. If
	// it is, don't do anything
	if len(pl.playlist) == len(uris) {
		equal := true
		for i, uri := range uris {
			if uri != pl.playlist[i].Uri() {
				equal = false
				break
			}
		}
		if equal {
			return false, nil
		}
	}

	pl.playlist = player.InterpolatePlaylistMeta(pl.playlist, player.TrackIdentities(uris...))
	return true, nil
}

// Initializes a track from an MPD hash. The hash should be gotten using
// ListAllInfo().
//
// ListAllInfo() and ListInfo() look very much the same but they don't return
// the same thing. Who the fuck thought it was a good idea to mix capitals and
// lowercase?!
func (pl *Player) trackFromMpdSong(song *mpd.Attrs, track *Track) {
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

	if timeStr := (*song)["Time"]; timeStr != "" {
		if duration, err := strconv.ParseInt(timeStr, 10, 32); err != nil {
			panic(err)
		} else {
			track.duration = time.Duration(duration) * time.Second
		}
	}
}

func (pl *Player) TrackInfo(identities ...player.TrackIdentity) ([]player.Track, error) {
	var tracks []player.Track
	err := pl.withMpd(func(mpdc *mpd.Client) error {
		var songs []mpd.Attrs
		var err error
		if len(identities) == 0 {
			songs, err = mpdc.ListAllInfo("/")
			if err != nil {
				return err
			}

		} else {
			songs = make([]mpd.Attrs, len(identities))
			for i, id := range identities {
				uri := id.Uri()
				if strings.HasPrefix(uri, URI_SCHEMA) {
					s, err := mpdc.ListAllInfo(uriToMpd(uri))
					if err != nil {
						return fmt.Errorf("Unable to get info about %v: %v", uri, err)
					}
					if len(s) > 0 {
						songs[i] = s[0]
						continue
					}
				} else if ok, _ := regexp.MatchString("https?:\\/\\/", uri); ok && len(pl.playlist) > 0 && pl.playlist[0].Uri() == uri {
					song, err := mpdc.CurrentSong()
					if err != nil {
						return fmt.Errorf("Unable to get info about %v: %v", uri, err)
					}
					songs[i] = song
					songs[i]["Album"] = song["Name"]
					continue
				}
				songs[i] = mpd.Attrs{"file": uri}
			}
		}

		numDirs := 0
		tracks = make([]player.Track, len(songs))
		for i, song := range songs {
			if _, ok := song["directory"]; ok {
				numDirs++
			} else {
				track := &Track{}
				pl.trackFromMpdSong(&song, track)
				tracks[i-numDirs] = track
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
		return nil
	})
	return tracks, err
}

func (pl *Player) Playlist() ([]player.PlaylistTrack, error) {
	pl.playlistLock.Lock()
	defer pl.playlistLock.Unlock()

	if len(pl.playlist) > 0 {
		// Update the progress attribute of the currently playing track.
		err := pl.withMpd(func(mpdc *mpd.Client) error {
			status, err := mpdc.Status()
			if err != nil {
				return err
			}

			progressf, _ := strconv.ParseFloat(status["elapsed"], 32)
			pl.playlist[0].Progress = time.Duration(progressf) * time.Second
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return pl.playlist, nil
}

func (pl *Player) SetPlaylist(plist []player.PlaylistTrack) error {
	pl.playlistLock.Lock()
	defer pl.playlistLock.Unlock()

	pl.playlist = plist

	return pl.withMpd(func(mpdc *mpd.Client) error {
		songs, err := mpdc.PlaylistInfo(-1, -1)
		if err != nil {
			return err
		}

		pl.playlistWasSet = true

		// Figure out how many tracks at the beginning of the playlist are unchanged.
		delStart := 0
		for len(songs) > delStart && len(pl.playlist) > delStart && uriToMpd(pl.playlist[delStart].Uri()) == songs[delStart]["file"] {
			delStart++
		}

		if delStart != len(songs) {
			// Clear the part of the playlist that does not match the new playlist.
			if err := mpdc.Delete(delStart, len(songs)); err != nil {
				return err
			}
		}

		// Queue the new tracks.
		cmd := mpdc.BeginCommandList()
		for _, track := range plist[delStart:] {
			cmd.Add(uriToMpd(track.Uri()))
		}
		return cmd.End()
	})
}

func (player *Player) Seek(progress time.Duration) error {
	return player.withMpd(func(mpdc *mpd.Client) error {
		status, err := mpdc.Status()
		if err != nil {
			return err
		}

		id, ok := statusAttrInt(status, "songid")
		if !ok {
			// No track is currently being played.
			return nil
		}
		return mpdc.SeekID(id, int(progress/time.Second))
	})
}

func (pl *Player) Next() error {
	return pl.withMpd(func(mpdc *mpd.Client) error {
		if err := mpdc.Next(); err != nil {
			return err
		}

		status, err := mpdc.Status()
		if err != nil {
			return err
		}
		if status["playlistlength"] != "0" {
			return mpdc.Delete(0, 1)
		} else {
			pl.Emit("playlist-end")
		}
		return nil
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

			// Don't attempt to start playback, just immediately end the
			// playlist.
			if status["playlistlength"] == "0" {
				pl.Emit("playlist-end")
				return nil
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

func (pl *Player) Available() bool {
	return pl.withMpd(func(mpdc *mpd.Client) error { return nil }) == nil
}

func (player *Player) Events() *util.Emitter {
	return player.Emitter
}

func incrementPlayCount(uri string, mpdc *mpd.Client) error {
	if !strings.HasPrefix(uri, URI_SCHEMA) {
		return nil
	}

	var playCount int64
	if str, err := mpdc.StickerGet(uriToMpd(uri), "play-count"); err == nil {
		playCount, _ = strconv.ParseInt(str, 10, 32)
	}
	if err := mpdc.StickerSet(uriToMpd(uri), "play-count", strconv.FormatInt(playCount+1, 10)); err != nil {
		return fmt.Errorf("Unable to set play-count: %v", err)
	}
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
