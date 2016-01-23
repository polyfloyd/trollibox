package slimserver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	player "../"
	"../../util"
)

type Player struct {
	ID    string
	Name  string
	Model string

	Serv *Server

	playlist       []player.PlaylistTrack
	playlistLock   sync.Mutex
	playlistWasSet bool
	lastTrack      string
	util.Emitter
}

func (pl *Player) eventLoop() {
	for {
		conn, _, err := pl.Serv.requestRaw("listen", "1")
		if err != nil {
			pl.Emit("availability")
			time.Sleep(time.Second)
			continue
		}

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), " ")
			for i, txt := range line {
				line[i], _ = url.QueryUnescape(txt)
			}
			if len(line) == 0 {
				continue
			}
			// Server global events.
			if len(line) >= 2 && line[0] == "rescan" && line[1] == "done" {
				pl.Emit("tracks")
			}

			if line[0] != pl.ID || len(line) < 2 {
				continue
			}
			// Events local to the player.
			switch {
			case line[1] == "playlist":
				if err := pl.reloadPlaylist(); err != nil {
					log.Println(err)
					continue
				}
				if len(line) >= 4 && line[2] == "newsong" && pl.lastTrack != line[3] {
					pl.lastTrack = line[3]

					currentTrackIndex, err := pl.currentTrackIndex()
					if err != nil {
						log.Println(err)
						continue
					}
					if len(pl.playlist) > 0 && currentTrackIndex >= 0 && pl.playlist[currentTrackIndex].Progress != 0 {
						if err := pl.Seek(-1, pl.playlist[currentTrackIndex].Progress); err != nil {
							log.Println(err)
							continue
						}
					}

					// It takes a while to get the metainformation from HTTP
					// streams. Emit another change event to inform that the
					// loading has been completed.
					pl.Emit("playlist")
				}
				if len(line) >= 2 && line[2] == "stop" {
					pl.maybeEmitPlaylistEnd()
				}

			case (line[1] == "play" || line[1] == "stop" || line[1] == "pause"):
				pl.Emit("playstate")

			case line[1] == "time":
				pl.Emit("progress")

			case line[1] == "mixer" && line[2] == "volume":
				pl.Emit("volume")

			case line[1] == "client":
				pl.Emit("availability")
			}
		}
		if err := scanner.Err(); err != nil {
			log.Println(err)
		}
		time.Sleep(time.Second)
	}
}

func (pl *Player) reloadPlaylist() error {
	pl.playlistLock.Lock()
	defer pl.playlistLock.Unlock()

	trackIds, err := pl.serverPlaylist()
	if err != nil {
		return err
	}
	// Check wether the argument playlist is equal to the stored playlist. If
	// it is, don't do anything
	playlistChanged := true
	if len(pl.playlist) == len(trackIds) {
		playlistChanged = false
		for i, id := range trackIds {
			if id != pl.playlist[i].TrackUri() {
				playlistChanged = true
				break
			}
		}
	}

	if playlistChanged || pl.playlistWasSet {
		pl.playlistWasSet = false
		trackIds, err := pl.serverPlaylist()
		if err != nil {
			return err
		}
		pl.playlist = player.InterpolatePlaylistMeta(pl.playlist, player.TrackIdentities(trackIds...))
		pl.Emit("playlist")
	}

	return nil
}

// Gets the ID's of the tracks in the SlimServer's playlist.
func (pl *Player) serverPlaylist() ([]string, error) {
	// Get the length of the playlist.
	res, err := pl.Serv.request(pl.ID, "playlist", "tracks", "?")
	if err != nil {
		return nil, err
	}
	playlistLength, _ := strconv.Atoi(res[3])

	// Get the URLs of the tracks in the playlist.
	trackIds := make([]string, playlistLength)
	for i := 0; i < playlistLength; i++ {
		res, err := pl.Serv.request(pl.ID, "playlist", "path", strconv.Itoa(i), "?")
		if err != nil {
			return nil, err
		}
		dec, _ := url.QueryUnescape(res[4])
		trackIds[i] = dec
	}
	return trackIds, nil
}

// Checks wether the playlist-end event should be emitted and fires it if no
// more tracks are available for playing.
func (pl *Player) maybeEmitPlaylistEnd() error {
	currentTrackIndex, err := pl.currentTrackIndex()
	if err != nil {
		return err
	}
	if currentTrackIndex == -1 {
		pl.Emit("playlist-end")
	}
	return nil
}

func (pl *Player) Tracks() ([]player.Track, error) {
	res, err := pl.Serv.request("info", "total", "songs", "?")
	if err != nil {
		return nil, err
	}
	numSongs, _ := strconv.Atoi(res[3])
	if numSongs == 0 {
		return []player.Track{}, nil
	}

	reader, release, err := pl.Serv.requestRaw("songs", "0", strconv.Itoa(numSongs), "tags:uAglitdc")
	if err != nil {
		return nil, err
	}
	defer release()

	scanner := bufio.NewScanner(reader)
	// Set a custom scanner to split on spaces and newlines. atEOF is ignored
	// since the reader does not end.
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, ' '); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, data[0:i], io.EOF
		}
		return 0, nil, nil
	})

	scanner.Scan() // "songs"
	scanner.Scan() // "0"
	scanner.Scan() // "n"
	scanner.Scan() // "tags"

	tracks := make([]player.Track, 0, numSongs)
	var track *player.Track
	for scanner.Scan() {
		tag, _ := url.QueryUnescape(scanner.Text())
		split := strings.SplitN(tag, ":", 2)

		if split[0] == "id" {
			if track != nil {
				tracks = append(tracks, *track)
			}
			track = &player.Track{}
		}
		setSlimAttr(pl.Serv, track, split[0], split[1])
	}
	tracks = append(tracks, *track)
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return tracks, nil
}

func (pl *Player) TrackInfo(identities ...player.TrackIdentity) ([]player.Track, error) {
	tracks := make([]player.Track, len(identities))
	for i, id := range identities {
		uri := id.TrackUri()

		isHttp, _ := regexp.MatchString("https?:\\/\\/", uri)
		if isHttp && len(pl.playlist) > 0 && pl.playlist[0].TrackUri() == uri {
			tr := &tracks[i]
			tr.Uri = uri
			tr.Album = uri
			artistRes, err := pl.Serv.request(pl.ID, "artist", "?")
			if err == nil && len(artistRes) >= 3 {
				tr.Artist = artistRes[2]
			}
			titleRes, err := pl.Serv.request(pl.ID, "title", "?")
			if err == nil && len(titleRes) >= 3 {
				tr.Title = titleRes[2]
			}
			player.InterpolateMissingFields(tr)

		} else if !isHttp {
			attrs, err := pl.Serv.requestAttrs("songinfo", "0", "100", "tags:uAglitdc", "url:"+encodeUri(id.TrackUri()))
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
			player.InterpolateMissingFields(&tracks[i])
		}
	}
	return tracks, nil
}

func (pl *Player) Playlist() ([]player.PlaylistTrack, int, error) {
	pl.playlistLock.Lock()
	defer pl.playlistLock.Unlock()

	currentTrackIndex, err := pl.currentTrackIndex()
	if err != nil {
		return nil, -1, err
	}

	plist := make([]player.PlaylistTrack, len(pl.playlist))
	copy(plist, pl.playlist)
	if len(plist) > 0 && currentTrackIndex >= 0 && currentTrackIndex < len(plist) {
		res, err := pl.Serv.request(pl.ID, "time", "?")
		if err != nil {
			return nil, -1, err
		}
		d, err := strconv.ParseFloat(res[2], 64)
		if err != nil {
			return nil, -1, err
		}
		plist[currentTrackIndex].Progress = time.Duration(d) * time.Second
	}
	return plist, currentTrackIndex, err
}

func (pl *Player) SetPlaylist(plist []player.PlaylistTrack) error {
	pl.playlistLock.Lock()
	defer pl.playlistLock.Unlock()
	pl.playlist = plist

	trackUrls, err := pl.serverPlaylist()
	if err != nil {
		return err
	}

	// Turn off the randommix plugin if it is active. Otherwise, it will refill
	// the playlist when we are removing tracks during the mutation.
	if _, err := pl.Serv.request(pl.ID, "randomplay", "disable"); err != nil {
		return err
	}

	// Calculate the index at which to start deleting.
	delStart := 0
	for len(trackUrls) > delStart && len(pl.playlist) > delStart {
		if pl.playlist[delStart].TrackUri() == trackUrls[delStart] {
			delStart++
		} else {
			break
		}
	}

	pl.playlistWasSet = true

	// Clear the part of the playlist that does not match the new playlist.
	if delStart != len(trackUrls) {
		for i := len(trackUrls); i >= delStart; i-- {
			pl.Serv.request(pl.ID, "playlist", "delete", strconv.Itoa(i))
		}
	}

	// Add the new tracks.
	for _, track := range plist[delStart:] {
		if _, err := pl.Serv.request(pl.ID, "playlist", "add", encodeUri(track.TrackUri())); err != nil {
			return err
		}
	}

	if delStart == 0 && len(plist) > 0 {
		return pl.SetState(player.PlayStatePlaying)
	}
	return nil
}

func (pl *Player) Seek(trackIndex int, offset time.Duration) error {
	if trackIndex != -1 {
		currentTrackIndex, err := pl.currentTrackIndex()
		if err != nil {
			return err
		}
		serverPlaylist, err := pl.serverPlaylist()
		if err != nil {
			return err
		}
		if currentTrackIndex == len(serverPlaylist)-1 {
			pl.Emit("playlist-end")
		} else {
			if _, err := pl.Serv.request(pl.ID, "playlist", "index", strconv.Itoa(trackIndex)); err != nil {
				return err
			}
		}
	}

	if offset != -1 {
		currentTrackIndex, err := pl.currentTrackIndex()
		if err != nil {
			return err
		}
		if currentTrackIndex != -1 {
			if _, err := pl.Serv.request(pl.ID, "time", strconv.Itoa(int(offset/time.Second))); err != nil {
				return err
			}
		}
	}
	return nil
}

func (pl *Player) State() (player.PlayState, error) {
	res, err := pl.Serv.request(pl.ID, "mode", "?")
	if err != nil {
		return player.PlayStateInvalid, err
	}
	switch res[2] {
	case "play":
		return player.PlayStatePlaying, nil
	case "paused":
		return player.PlayStatePaused, nil
	case "stop":
		return player.PlayStateStopped, nil
	default:
		return player.PlayStateInvalid, nil
	}
}

func (pl *Player) SetState(state player.PlayState) error {
	switch state {
	case player.PlayStatePlaying:
		if err := pl.maybeEmitPlaylistEnd(); err != nil {
			return err
		}
		_, err := pl.Serv.request(pl.ID, "play")
		return err
	case player.PlayStatePaused:
		_, err := pl.Serv.request(pl.ID, "pause", "1")
		return err
	case player.PlayStateStopped:
		_, err := pl.Serv.request(pl.ID, "stop")
		return err
	}
	return fmt.Errorf("Invalid playstate")
}

func (pl *Player) Volume() (float32, error) {
	res, err := pl.Serv.request(pl.ID, "mixer", "volume", "?")
	if err != nil {
		return 0, err
	}
	vol, _ := strconv.ParseInt(res[3], 10, 32)
	if vol < 0 {
		// The volume is negative if the player is muted.
		return 0, nil
	}
	return float32(vol) / 100, nil
}

func (pl *Player) SetVolume(vol float32) error {
	// Also unmute the in case the player was muted.
	_, err := pl.Serv.request(pl.ID, "mixer", "muting", "0")
	if err != nil {
		return err
	}
	_, err = pl.Serv.request(pl.ID, "mixer", "volume", strconv.Itoa(int(vol*100)))
	return err
}

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

func (pl *Player) TrackArt(track player.TrackIdentity) (image io.ReadCloser, mime string) {
	attrs, err := pl.Serv.requestAttrs("songinfo", "0", "100", "tags:c", "url:"+encodeUri(track.TrackUri()))
	if err != nil {
		return nil, ""
	}
	if pl.Serv.webUrl == "" || attrs["coverid"] == "" {
		return nil, ""
	}
	res, err := http.Get(fmt.Sprintf("%smusic/%s/cover.jpg", pl.Serv.webUrl, attrs["coverid"]))
	if err != nil {
		return nil, ""
	}
	return res.Body, res.Header.Get("Content-Type")
}

func (pl *Player) Events() *util.Emitter {
	return &pl.Emitter
}

func (pl *Player) currentTrackIndex() (int, error) {
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

func (pl *Player) String() string {
	return fmt.Sprintf("%s [%s] [%s]", pl.Name, pl.ID, pl.Model)
}

func setSlimAttr(serv *Server, track *player.Track, key, value string) {
	switch key {
	case "url":
		uri, _ := url.QueryUnescape(value)
		track.Uri = uri
	case "artist":
		fallthrough
	case "trackartist":
		track.Artist = value
	case "title":
		track.Title = value
	case "genre":
		track.Genre = value
	case "album":
		if a := value; a != "No Album" {
			track.Album = a
		}
	case "albumartist":
		track.AlbumArtist = value
	case "tracknum":
		track.AlbumTrack = value
	case "disc":
		track.AlbumDisc = value
	case "duration":
		d, _ := strconv.ParseFloat(value, 64)
		track.Duration = time.Duration(d) * time.Second
	case "coverid":
		track.HasArt = serv.webUrl != "" && value != ""
	}
}
