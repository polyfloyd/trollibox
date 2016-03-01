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
	"sort"
	"strconv"
	"strings"
	"time"

	player "../"
	"../../util"
)

type Player struct {
	ID    string
	Name  string
	Model string

	Serv *Server

	playlist player.PlaylistMetaKeeper

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
				continue
			}

			if line[0] != pl.ID || len(line) < 2 {
				continue
			}
			// Events local to the player.
			switch {
			case line[1] == "playlist":
				if len(line) >= 3 && line[2] == "newsong" {
					// It takes a while to get the metainformation from HTTP
					// streams. Emit another change event to inform that the
					// loading has been completed.
					pl.Emit("playlist")
				}
				if len(line) >= 3 && (line[2] == "load_done" || line[2] == "move" || line[2] == "delete") {
					pl.Emit("playlist")
				}

			case (line[1] == "play" || line[1] == "stop" || line[1] == "pause"):
				pl.Emit("playstate")

			case line[1] == "time":
				pl.Emit("time")

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

func (pl *Player) playlistLength() (int, error) {
	res, err := pl.Serv.request(pl.ID, "playlist", "tracks", "?")
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(res[3])
}

func (pl *Player) killRandomPlay() error {
	_, err := pl.Serv.request(pl.ID, "randomplay", "disable")
	return err
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

func (pl *Player) TrackInfo(uris ...string) ([]player.Track, error) {
	res, err := pl.Serv.request(pl.ID, "path", "?")
	if err != nil {
		return nil, err
	}
	var currentTrackUri string
	if len(res) >= 3 {
		currentTrackUri, _ = url.QueryUnescape(res[2])
	}

	tracks := make([]player.Track, len(uris))
	for i, uri := range uris {
		isHttp, _ := regexp.MatchString("https?:\\/\\/", uri)
		if isHttp && currentTrackUri == uri {
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
			attrs, err := pl.Serv.requestAttrs("songinfo", "0", "100", "tags:uAglitdc", "url:"+encodeUri(uri))
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

func (pl *Player) SetTime(offset time.Duration) error {
	_, err := pl.Serv.request(pl.ID, "time", strconv.Itoa(int(offset/time.Second)))
	return err
}

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

func (pl *Player) SetTrackIndex(trackIndex int) error {
	if plistLen, err := pl.Playlist().Len(); err != nil {
		return err
	} else if trackIndex >= plistLen {
		return pl.SetState(player.PlayStateStopped)
	}
	_, err := pl.Serv.request(pl.ID, "playlist", "index", strconv.Itoa(trackIndex))
	return err
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

func (pl *Player) Playlist() player.MetaPlaylist {
	return &pl.playlist
}

func (pl *Player) TrackArt(track string) (image io.ReadCloser, mime string) {
	attrs, err := pl.Serv.requestAttrs("songinfo", "0", "100", "tags:c", "url:"+encodeUri(track))
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

func (pl *Player) String() string {
	return fmt.Sprintf("Slim{%s, %s, %s}", pl.Name, pl.ID, pl.Model)
}

type slimPlaylist struct {
	player *Player
}

func (plist slimPlaylist) Insert(pos int, tracks ...player.Track) error {
	plist.player.killRandomPlay()
	if pos == -1 {
		for _, track := range tracks {
			if _, err := plist.player.Serv.request(plist.player.ID, "playlist", "add", encodeUri(track.Uri)); err != nil {
				return err
			}
		}
	} else {
		// TODO
		return fmt.Errorf("UNIMPLEMENTED")
	}
	return nil
}

func (plist slimPlaylist) Move(fromPos, toPos int) error {
	plist.player.killRandomPlay()
	_, err := plist.player.Serv.request(plist.player.ID, "playlist", "move", strconv.Itoa(fromPos), strconv.Itoa(toPos))
	return err
}

func (plist slimPlaylist) Remove(positions ...int) error {
	plist.player.killRandomPlay()
	sort.Ints(positions)
	for i := len(positions) - 1; i >= 0; i-- {
		if _, err := plist.player.Serv.request(plist.player.ID, "playlist", "delete", strconv.Itoa(positions[i])); err != nil {
			return err
		}
	}
	return nil
}

func (plist slimPlaylist) Tracks() ([]player.Track, error) {
	trackUris, err := plist.player.serverPlaylist()
	if err != nil {
		return nil, err
	}
	tracks, err := plist.player.TrackInfo(trackUris...)
	if err != nil {
		return nil, err
	}
	plTracks := make([]player.Track, len(tracks))
	for i, tr := range tracks {
		plTracks[i] = tr
	}
	return plTracks, err
}

func (plist slimPlaylist) Len() (int, error) {
	tracks, err := plist.Tracks()
	if err != nil {
		return -1, err
	}
	return len(tracks), err
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
