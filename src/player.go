package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strconv"

	"github.com/polyfloyd/gompd/mpd"
)

type Track interface {
	GetUri() string
}

type LocalTrack struct {
	player *Player

	Id          string `json:"id"`
	Artist      string `json:"artist,omitempty"`
	Title       string `json:"title,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"albumartist,omitempty"`
	AlbumTrack  string `json:"albumtrack,omitempty"`
	AlbumDisc   string `json:"albumdisc,omitempty"`
	Duration    int    `json:"duration"`
}

func (track *LocalTrack) GetUri() string {
	return track.Id
}

func (track *LocalTrack) AttributeByName(attr string) interface{} {
	switch attr {
	case "id":
		fallthrough
	case "uri":
		return track.Id
	case "artist":
		return track.Artist
	case "title":
		return track.Title
	case "genre":
		return track.Genre
	case "album":
		return track.AlbumArtist
	case "albumartist":
		return track.AlbumArtist
	case "albumtrack":
		return track.AlbumTrack
	case "albumdisc":
		return track.AlbumDisc
	case "duration":
		return track.Duration
	}
	return nil
}

func (track *LocalTrack) GetArt() (image io.Reader) {
	track.player.withMpd(func(mpdc *mpd.Client) {
		numChunks := 0
		if strNum, err := mpdc.StickerGet(track.Id, "image-nchunks"); err == nil {
			if num, err := strconv.ParseInt(strNum, 10, 32); err == nil {
				numChunks = int(num)
			}
		}
		if numChunks == 0 {
			return
		}

		var chunks []io.Reader
		for i := 0; i < numChunks; i++ {
			if b64Data, err := mpdc.StickerGet(track.Id, fmt.Sprintf("image-%v", i)); err != nil {
				return
			} else {
				chunks = append(chunks, bytes.NewReader([]byte(b64Data)))
			}
		}
		image = base64.NewDecoder(base64.StdEncoding, io.MultiReader(chunks...))
	})
	return
}

func (track *LocalTrack) HasArt() (hasArt bool) {
	track.player.withMpd(func(mpdc *mpd.Client) {
		_, err := mpdc.StickerGet(track.Id, "image-nchunks")
		hasArt = err == nil
	})
	return
}

type QueueAttrs struct {
	QueuedBy string `json:"queuedby"`
}

type PlaylistTrack struct {
	Track
	QueueAttrs
}

type Player struct {
	*EventEmitter

	// Running the idle routine on the same connection as the main connection
	// will fuck things up badly.
	mpdWatcher *mpd.Watcher

	addr, passwd string

	// A map containing properties related to tracks currently in the queue.
	queueAttrs map[string]QueueAttrs

	// Sometimes, the volume returned by MDP is invalid, so we have to take
	// care of that ourselves.
	lastVolume float32

	// We use this value to determine wether the currently playing track has
	// changed.
	lastTrack string

	streamdb *StreamDB
	queuer   *Queuer
}

func NewPlayer(mpdHost string, mpdPort int, mpdPassword *string, streamdb *StreamDB, queuer *Queuer) (*Player, error) {
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
		EventEmitter: NewEventEmitter(),
		mpdWatcher:   clientWatcher,
		queueAttrs:   map[string]QueueAttrs{},

		addr:   addr,
		passwd: passwd,

		streamdb: streamdb,
		queuer:   queuer,
	}

	go player.eventLoop()
	go player.queueLoop()
	go player.playlistLoop()

	return player, nil
}

func (player *Player) withMpd(fn func(mpd *mpd.Client)) {
	client, err := mpd.DialAuthenticated("tcp", player.addr, player.passwd)
	if err != nil {
		log.Println(err)
		player.withMpd(fn)
		return
	}
	defer client.Close()
	fn(client)
}

func (player *Player) eventLoop() {
	streamdbCh := player.StreamDB().Listen()
	defer player.queuer.Unlisten(streamdbCh)
	queuerCh := player.queuer.Listen()
	defer player.queuer.Unlisten(queuerCh)

	for {
		select {
		case event := <-player.mpdWatcher.Event:
			player.Emit(event)
		case event := <-streamdbCh:
			player.Emit("streams-" + event)
		case event := <-queuerCh:
			player.Emit("queuer-" + event)
		case err := <-player.mpdWatcher.Error:
			log.Println(err)
		}
	}
}

func (player *Player) queueLoop() {
	listener := player.Listen()
	defer player.Unlisten(listener)
	listener <- "player" // Bootstrap the cycle
	for {
		if event := <-listener; event != "player" {
			continue
		}

		player.withMpd(func(mpdc *mpd.Client) {
			// Remove played tracks form the queue.
			status, err := mpdc.Status()
			if err != nil {
				log.Println(err)
				return
			}
			songIndex := 0
			if str, ok := status["song"]; ok {
				if song64, err := strconv.ParseInt(str, 10, 32); err == nil {
					songIndex = int(song64)
				} else {
					log.Println(err)
				}
			} else if status["state"] == "stop" && status["playlistlength"] != "0" {
				// Fix to make sure the previous track is not played twice.
				// TODO: Some funny stuff happens when MPD receives a stop command.
				songIndex = 1
			}
			if songIndex != 0 {
				if err := mpdc.Delete(0, songIndex); err != nil {
					log.Println(err)
				}
			}

			// Queue a new track if nothing is playing.
			track, _, err := player.CurrentTrack()
			if err != nil {
				log.Println(err)
				return
			}
			if track == nil {
				if err := player.QueueRandom(); err != nil {
					log.Println(err)
					return
				}
				if err = player.SetState("playing"); err != nil {
					log.Println(err)
					return
				}
			}
		})
	}
}

func (player *Player) playlistLoop() {
	listener := player.Listen()
	defer player.Unlisten(listener)
	listener <- "playlist" // Bootstrap the cycle
	for {
		if event := <-listener; event != "playlist" {
			continue
		}

		player.withMpd(func(mpdc *mpd.Client) {
			songs, err := mpdc.PlaylistInfo(-1, -1)
			if err != nil {
				log.Println(err)
				return
			}

			// Synchronize queue attributes.
			// Remove tracks that are no longer in the list
		trackRemoveLoop:
			for id := range player.queueAttrs {
				for _, song := range songs {
					if song["file"] == id {
						continue trackRemoveLoop
					}
				}
				delete(player.queueAttrs, id)
			}

			// Initialize tracks that were wiped due to restarts or not added using
			// Trollibox.
		trackInitLoop:
			for _, song := range songs {
				for id := range player.queueAttrs {
					if song["file"] == id {
						continue trackInitLoop
					}
				}
				player.queueAttrs[song["file"]] = QueueAttrs{
					// Assume the track was queued by a human.
					QueuedBy: "user",
				}
			}

			if len(songs) > 0 {
				currentUri := songs[0]["file"]

				// TODO: If one track is followed by another track with the same
				// ID, the next block will not be executed, leaving the playcount
				// unchanged.
				if player.lastTrack != currentUri {
					// Streams can't have stickers.
					if !IsStreamUri(currentUri) {
						// Increment the playcount for player track.
						var playCount int64
						if str, err := mpdc.StickerGet(currentUri, "play-count"); err == nil {
							playCount, _ = strconv.ParseInt(str, 10, 32)
						}
						if err := mpdc.StickerSet(currentUri, "play-count", strconv.FormatInt(playCount+1, 10)); err != nil {
							log.Printf("Could not set play-count: %v", err)
						}
					}

					player.lastTrack = currentUri
				}
			}
		})
	}
}

func (player *Player) localTrackFromMpdSong(song *mpd.Attrs, track *LocalTrack, mpdc *mpd.Client) {
	track.player = player

	if _, ok := (*song)["directory"]; ok {
		panic("Tried to read a directory as local file")
	}

	track.Id = (*song)["file"]
	if IsStreamUri(track.Id) {
		panic("Tried to read a stream as local file")
	}

	track.Artist = (*song)["Artist"]
	track.Title = (*song)["Title"]
	track.Genre = (*song)["Genre"]
	track.Album = (*song)["Album"]
	track.AlbumArtist = (*song)["AlbumArtist"]
	track.AlbumDisc = (*song)["Disc"]
	track.AlbumTrack = (*song)["Track"]

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
			track.Duration = int(duration)
		}
	}
}

func (player *Player) streamTrackFromMpdSong(song *mpd.Attrs, stream *StreamTrack, mpdc *mpd.Client) {
	if tmpl := player.StreamDB().StreamByURL((*song)["file"]); tmpl != nil {
		// Make a copy to prevent polluting the original.
		*stream = StreamTrack(*tmpl)
	}
	if name, ok := (*song)["Name"]; ok {
		stream.Album = name
	}
	if stream.Album == "" {
		stream.Album = stream.Url
	}
	stream.Title = (*song)["Title"]
}

func (player *Player) playlistTrackFromMpdSong(song *mpd.Attrs, track *PlaylistTrack, mpdc *mpd.Client) {
	if IsStreamUri((*song)["file"]) {
		var streamTrack StreamTrack
		player.streamTrackFromMpdSong(song, &streamTrack, mpdc)
		track.Track = &streamTrack
	} else {
		var tr LocalTrack
		player.localTrackFromMpdSong(song, &tr, mpdc)
		track.Track = &tr
	}
	track.QueueAttrs = player.queueAttrs[track.Track.GetUri()]
}

func (player *Player) Queue(uri string, queuedBy string) (err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		player.queueAttrs[uri] = QueueAttrs{
			QueuedBy: queuedBy,
		}
		err = mpdc.Add(uri)
	})
	return
}

func (player *Player) QueueRandom() error {
	tracks, err := player.ListTracks("", true)
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return nil
	}

	track := player.queuer.SelectTrack(tracks)
	if track == nil {
		log.Println("No tracks passed queue criteria")
		track = player.queuer.RandomTrack(tracks)
	}

	return player.Queue(track.GetUri(), "system")
}

func (player *Player) Volume() (vol float32, err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		var status mpd.Attrs
		status, err = mpdc.Status()
		if err != nil {
			return
		}

		volStr, ok := status["volume"]
		if !ok {
			// Volume should always be present
			err = fmt.Errorf("No volume property is present in the MPD status")
			return
		}

		var rawVol int64
		rawVol, err = strconv.ParseInt(volStr, 10, 32)
		if err != nil {
			return
		}

		vol = float32(rawVol) / 100
		// Happens sometimes when nothing is playing
		if vol < 0 {
			vol = player.lastVolume
		}
	})
	return
}

func (player *Player) SetVolume(vol float32) (err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		if vol > 1 {
			vol = 1
		} else if vol < 0 {
			vol = 0
		}

		player.lastVolume = vol
		err = mpdc.SetVolume(int(vol * 100))
	})
	return
}

func (player *Player) ListTracks(path string, recursive bool) (tracks []LocalTrack, err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		if path == "" {
			path = "/"
		}

		var songs []mpd.Attrs
		if recursive {
			songs, err = mpdc.ListAllInfo(path)
		} else {
			songs, err = mpdc.ListInfo(path)
		}
		if err != nil {
			return
		}

		numDirs := 0
		tracks = make([]LocalTrack, len(songs))
		for i, song := range songs {
			if _, ok := song["directory"]; ok {
				numDirs++
			} else {
				player.localTrackFromMpdSong(&song, &tracks[i-numDirs], mpdc)
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
	})
	return
}

func (player *Player) Playlist() (tracks []PlaylistTrack, err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		var songs []mpd.Attrs
		songs, err = mpdc.PlaylistInfo(-1, -1)
		if err != nil {
			return
		}

		tracks = make([]PlaylistTrack, len(songs))
		for i, song := range songs {
			player.playlistTrackFromMpdSong(&song, &tracks[i], mpdc)
		}
	})
	return
}

func (player *Player) SetPlaylistIds(trackIds []string) error {
	playlist, err := player.Playlist()
	if err != nil {
		return err
	}

	player.withMpd(func(mpd *mpd.Client) {
		// Playing track is not the first track of the new list? Remove it so we
		// can overwrite it.
		var delStart int
		if playlist[0].GetUri() == trackIds[0] {
			// Don't queue the first track twice.
			delStart = 1
			trackIds = trackIds[1:]
		} else {
			delStart = 0
		}

		// Clear the playlist
		if delStart != len(playlist) {
			if err = mpd.Delete(delStart, len(playlist)); err != nil {
				return
			}
		}

		// Queue the new tracks.
		cmd := mpd.BeginCommandList()
		for _, id := range trackIds {
			cmd.Add(id)
		}
		err = cmd.End()
	})
	return err
}

// Returns the currently playing track as well as its progress in seconds
func (player *Player) CurrentTrack() (track *PlaylistTrack, progress int, err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		var status mpd.Attrs
		status, err = mpdc.Status()
		if err != nil {
			return
		}

		if st, ok := status["state"]; !ok || st == "stop" {
			return
		}

		var playlist []PlaylistTrack
		playlist, err = player.Playlist()
		if err != nil || len(playlist) == 0 {
			return
		}
		track = &playlist[0]

		var progressf float64
		progressf, err = strconv.ParseFloat(status["elapsed"], 32)
		if err != nil {
			progressf = 0
		}
		progress = int(progressf)
	})
	return
}

func (player *Player) SetProgress(progress int) (err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		var status mpd.Attrs
		status, err = mpdc.Status()
		if err != nil {
			return
		}

		if str, ok := status["songid"]; !ok {
			// No track is currently being played.
		} else if id, perr := strconv.ParseInt(str, 10, 32); err != nil {
			err = perr
		} else {
			err = mpdc.SeekID(int(id), progress)
		}
	})
	return
}

func (player *Player) Next() (err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		if err = mpdc.Next(); err != nil {
			return
		}
		err = mpdc.Delete(0, 1)
	})
	return
}

func (player *Player) State() (state string, err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		var status mpd.Attrs
		status, err = mpdc.Status()
		if err != nil {
			return
		}

		state = map[string]string{
			"play":  "playing",
			"pause": "paused",
			"stop":  "stopped",
		}[status["state"]]
	})
	return
}

func (player *Player) SetState(state string) (err error) {
	player.withMpd(func(mpdc *mpd.Client) {
		switch state {
		case "paused":
			err = mpdc.Pause(true)
		case "playing":
			var status mpd.Attrs
			if status, err = mpdc.Status(); err == nil {
				if status["state"] == "stop" {
					err = mpdc.Play(0)
				} else {
					err = mpdc.Pause(false)
				}
			}
		case "stopped":
			err = mpdc.Stop()
		default:
			err = fmt.Errorf("Unknown play state %v", state)
		}
	})
	return
}

func (player *Player) StreamDB() *StreamDB {
	return player.streamdb
}

func (player *Player) Queuer() *Queuer {
	return player.queuer
}
