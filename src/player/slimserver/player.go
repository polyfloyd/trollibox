package slimserver

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/library/cache"
	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/util"
)

const trackTags = "uAglitdc"

var eventTranslations = []struct {
	Exp   *regexp.Regexp
	Event func(*Player, []string) (player.Event, error)
	// If the global bit is not set, the expression is ignored if the event
	// line does not start with the player's ID.
	Global bool
}{
	{
		Exp: regexp.MustCompile("^rescan done"),
		Event: func(pl *Player, m []string) (player.Event, error) {
			return library.UpdateEvent{}, nil
		},
		Global: true,
	},
	{
		Exp: regexp.MustCompile(`^\S+ mixer volume (\d+)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			volume, _ := strconv.Atoi(m[1])
			if volume > 100 {
				volume = 100
			}
			return player.VolumeEvent{Volume: volume}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ prefset server volume (\d+)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			volume, _ := strconv.Atoi(m[1])
			return player.VolumeEvent{Volume: volume}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ prefset server currentSong (\d+)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			index, _ := strconv.Atoi(m[1])
			return player.PlaylistEvent{Index: index}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ prefset server currentSong (\d+)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			return player.PlayStateEvent{State: player.PlayStatePlaying}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ playlist (?:delete|newsong)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			index, err := pl.TrackIndex()
			if err != nil {
				return nil, err
			}
			return player.PlaylistEvent{Index: index}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ playlist pause (0|1)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			var state player.PlayState
			if m[1] == "0" {
				state = player.PlayStatePlaying
			} else {
				state = player.PlayStatePaused
			}
			return player.PlayStateEvent{State: state}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ playlist play`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			return player.PlayStateEvent{State: player.PlayStatePlaying}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ playlist stop`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			return player.PlayStateEvent{State: player.PlayStateStopped}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ time (\d+)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			secs, _ := strconv.Atoi(m[1])
			return player.TimeEvent{Time: time.Second * time.Duration(secs)}, nil
		},
	},
	{
		Exp: regexp.MustCompile(`^\S+ client (?:reconnect|disconnect)`),
		Event: func(pl *Player, m []string) (player.Event, error) {
			return player.AvailabilityEvent{Available: m[1] == "reconnect"}, nil
		},
	},
}

// A Player that is part of a Server.
type Player struct {
	ID    string
	Name  string
	Model string

	Serv *Server

	cachedLibrary *cache.Cache
	playlist      player.PlaylistMetaKeeper

	util.Emitter
}

func (pl *Player) eventLoop() {
	for {
		conn, _, err := pl.Serv.requestRaw("listen", "1")
		if err != nil {
			log.Debugf("Could not start event loop: %v", err)
			pl.Emit(player.AvailabilityEvent{Available: false})
			time.Sleep(time.Second)
			continue
		}

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line, err := url.QueryUnescape(scanner.Text())
			if err != nil {
				log.WithField("line", scanner.Text()).
					Errorf("Could not parse line from event loop: %v", err)
				continue
			} else if len(line) == 0 {
				continue
			}

			for _, evtr := range eventTranslations {
				if !evtr.Global && !strings.HasPrefix(line, pl.ID) {
					continue
				}
				if m := evtr.Exp.FindStringSubmatch(line); m != nil {
					event, err := evtr.Event(pl, m)
					if err != nil {
						log.WithField("line", scanner.Text()).
							Errorf("Could build event: %v", err)
						break
					}
					pl.Emit(event)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("Could not scan event loop: %v", err)
		}
	}
}

// Library implements the player.Player interface.
func (pl *Player) Library() library.Library {
	return pl.cachedLibrary
}

// Tracks implements the library.Library interface.
func (pl *Player) Tracks() ([]library.Track, error) {
	res, err := pl.Serv.request("info", "total", "songs", "?")
	if err != nil {
		return nil, err
	}
	numTracks, _ := strconv.Atoi(res[3])
	return pl.Serv.decodeTracks("id", numTracks, "songs", "0", strconv.Itoa(numTracks), "tags:"+trackTags)
}

// TrackInfo implements the library.Library interface.
func (pl *Player) TrackInfo(uris ...string) ([]library.Track, error) {
	res, err := pl.Serv.request(pl.ID, "path", "?")
	if err != nil {
		return nil, err
	}
	var currentTrackURI string
	if len(res) >= 3 {
		currentTrackURI, _ = url.QueryUnescape(res[2])
	}

	tracks := make([]library.Track, len(uris))
	for i, uri := range uris {
		isHTTP := strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "http://")
		if isHTTP && currentTrackURI == uri {
			tr := &tracks[i]
			tr.URI = uri
			tr.Album = uri
			artistRes, err := pl.Serv.request(pl.ID, "artist", "?")
			if err == nil && len(artistRes) >= 3 {
				tr.Artist = artistRes[2]
			}
			titleRes, err := pl.Serv.request(pl.ID, "title", "?")
			if err == nil && len(titleRes) >= 3 {
				tr.Title = titleRes[2]
			}
			library.InterpolateMissingFields(tr)
			continue
		}

		if !isHTTP {
			attrs, err := pl.Serv.requestAttrs("songinfo", "0", "100", "tags:"+trackTags, "url:"+encodeURI(uri))
			if err != nil {
				return nil, err
			}
			// Skip tracks that are not known to the server.
			if _, ok := attrs["duration"]; !ok {
				continue
			}

			for k, v := range attrs {
				setSlimAttr(pl.Serv, &tracks[i], k, v)
			}
			library.InterpolateMissingFields(&tracks[i])
		}
	}
	return tracks, nil
}

// Time implements the player.Player interface.
func (pl *Player) Time() (time.Duration, error) {
	res, err := pl.Serv.request(pl.ID, "time", "?")
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(res[2], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(d) * time.Second, nil
}

// SetTime implements the player.Player interface.
func (pl *Player) SetTime(offset time.Duration) error {
	_, err := pl.Serv.request(pl.ID, "time", strconv.Itoa(int(offset/time.Second)))
	return err
}

// TrackIndex implements the player.Player interface.
func (pl *Player) TrackIndex() (int, error) {
	numTrackRes, err := pl.Serv.request(pl.ID, "playlist", "tracks", "?")
	if err != nil || numTrackRes[3] == "0" {
		return -1, err
	}
	state, err := pl.State()
	if err != nil || state == player.PlayStateStopped {
		return -1, err
	}
	res, err := pl.Serv.request(pl.ID, "playlist", "index", "?")
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(res[3])
}

// SetTrackIndex implements the player.Player interface.
func (pl *Player) SetTrackIndex(trackIndex int) error {
	if plistLen, err := pl.Playlist().Len(); err != nil {
		return err
	} else if trackIndex >= plistLen {
		return pl.SetState(player.PlayStateStopped)
	}
	_, err := pl.Serv.request(pl.ID, "playlist", "index", strconv.Itoa(trackIndex))
	return err
}

// State implements the player.Player interface.
func (pl *Player) State() (player.PlayState, error) {
	res, err := pl.Serv.request(pl.ID, "mode", "?")
	if err != nil {
		return player.PlayStateInvalid, err
	}
	switch res[2] {
	case "play":
		return player.PlayStatePlaying, nil
	case "pause":
		return player.PlayStatePaused, nil
	case "stop":
		return player.PlayStateStopped, nil
	default:
		return player.PlayStateInvalid, fmt.Errorf("server returned an invalid playstate: %q", res[2])
	}
}

// SetState implements the player.Player interface.
func (pl *Player) SetState(state player.PlayState) error {
	ack := make(chan error, 1)
	defer close(ack)
	// SlimServer may have acknowledged the command, but has not processed it.
	// This could result in State() returning the wrong value, if it were to be
	// called immediately after SetState. Wait for the playstate event to be
	// emitted before continuing.
	go func() {
		events := pl.Listen()
		defer pl.Unlisten(events)
		timeout := time.After(time.Second * 8)
	outer:
		for {
			select {
			case e := <-events:
				if _, ok := e.(player.PlayStateEvent); ok {
					ack <- nil
					break outer
				}
			case <-timeout:
				ack <- fmt.Errorf("timeout waiting for playstate update")
				break outer
			}
		}
	}()

	var err error
	switch state {
	case player.PlayStatePlaying:
		_, err = pl.Serv.request(pl.ID, "mode", "play")
	case player.PlayStatePaused:
		_, err = pl.Serv.request(pl.ID, "mode", "pause")
	case player.PlayStateStopped:
		_, err = pl.Serv.request(pl.ID, "mode", "stop")
	default:
		err = fmt.Errorf("attempted to set an invalid playstate: %q", state)
	}
	if err != nil {
		return err
	}
	return <-ack
}

// Volume implements the player.Player interface.
func (pl *Player) Volume() (int, error) {
	res, err := pl.Serv.request(pl.ID, "mixer", "volume", "?")
	if err != nil {
		return 0, err
	}
	vol, _ := strconv.Atoi(res[3])
	if vol < 0 {
		// The volume is negative if the player is muted.
		return 0, nil
	}
	return vol, nil
}

// SetVolume implements the player.Player interface.
func (pl *Player) SetVolume(vol int) error {
	// Also unmute the in case the player was muted.
	_, err := pl.Serv.request(pl.ID, "mixer", "muting", "0")
	if err != nil {
		return err
	}
	_, err = pl.Serv.request(pl.ID, "mixer", "volume", strconv.Itoa(vol))
	return err
}

// Lists implements the player.Player interface.
func (pl *Player) Lists() (map[string]player.Playlist, error) {
	countRes, err := pl.Serv.requestAttrs("playlists")
	if err != nil {
		return nil, err
	}
	numPlaylists, err := strconv.Atoi(countRes["count"])
	if err != nil {
		return nil, err
	}

	playlists := map[string]player.Playlist{}
	for i := 0; i < numPlaylists; i++ {
		plAttrs, err := pl.Serv.requestAttrs("playlists", strconv.Itoa(i), "1")
		if err != nil {
			return nil, err
		}
		playlists[plAttrs["playlist"]] = userPlaylist{
			player: pl,
			id:     plAttrs["id"],
		}
	}
	return playlists, nil
}

// Available implements the player.Player interface.
func (pl *Player) Available() bool {
	powerRes, err := pl.Serv.request(pl.ID, "power", "?")
	if err != nil {
		return false
	}
	connectedRes, err := pl.Serv.request(pl.ID, "connected", "?")
	if err != nil {
		return false
	}
	return powerRes[2] == "1" && connectedRes[2] == "1"
}

// Playlist implements the player.Player interface.
func (pl *Player) Playlist() player.MetaPlaylist {
	return &pl.playlist
}

// TrackArt implements the library.Library interface.
func (pl *Player) TrackArt(track string) (image io.ReadCloser, mime string) {
	attrs, err := pl.Serv.requestAttrs("songinfo", "0", "100", "tags:c", "url:"+encodeURI(track))
	if err != nil {
		return nil, ""
	}
	if pl.Serv.webURL == "" || attrs["coverid"] == "" {
		return nil, ""
	}
	res, err := http.Get(fmt.Sprintf("%smusic/%s/cover.jpg", pl.Serv.webURL, attrs["coverid"]))
	if err != nil {
		return nil, ""
	}
	if res.StatusCode >= 400 {
		return nil, ""
	}
	return res.Body, res.Header.Get("Content-Type")
}

// Events implements the player.Player interface.
func (pl *Player) Events() *util.Emitter {
	return &pl.Emitter
}

func (pl *Player) String() string {
	return fmt.Sprintf("Slim{%s, %s, %s}", pl.Name, pl.ID, pl.Model)
}

type slimPlaylist struct {
	player *Player
}

func (plist slimPlaylist) Insert(pos int, tracks ...library.Track) error {
	originalLength, err := plist.Len()
	if err != nil {
		return err
	}

	// Append to the end.
	for _, track := range tracks {
		_, err := plist.player.Serv.request(plist.player.ID, "playlist", "add", encodeURI(track.URI))
		if err != nil {
			return err
		}
	}
	if pos == -1 || originalLength == 0 {
		return nil
	}
	// SlimServer does not support inserting at a specific position, so
	// We'll just have to move it ourselves.
	for i := range tracks {
		if err := plist.Move(originalLength+i, pos+i); err != nil {
			return err
		}
	}
	return nil
}

func (plist slimPlaylist) Move(fromPos, toPos int) error {
	_, err := plist.player.Serv.request(plist.player.ID, "playlist", "move", strconv.Itoa(fromPos), strconv.Itoa(toPos))
	return err
}

func (plist slimPlaylist) Remove(positions ...int) error {
	sort.Ints(positions)
	for i := len(positions) - 1; i >= 0; i-- {
		if _, err := plist.player.Serv.request(plist.player.ID, "playlist", "delete", strconv.Itoa(positions[i])); err != nil {
			return err
		}
	}
	return nil
}

func (plist slimPlaylist) Tracks() ([]library.Track, error) {
	res, err := plist.player.Serv.request("info", "total", "songs", "?")
	if err != nil {
		return nil, err
	}
	numTracks, err := strconv.Atoi(res[3])
	if err != nil {
		return nil, err
	}
	return plist.player.Serv.decodeTracks("id", numTracks, plist.player.ID, "status", "0", strconv.Itoa(numTracks), "tags:"+trackTags)
}

func (plist slimPlaylist) Len() (int, error) {
	res, err := plist.player.Serv.request(plist.player.ID, "playlist", "tracks", "?")
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(res[3])
}
