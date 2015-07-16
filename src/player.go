package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strconv"
	mpd "github.com/polyfloyd/gompd/mpd"
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

func (this *LocalTrack) GetUri() string {
	return this.Id
}

func (this *LocalTrack) AttributeByName(attr string) interface{} {
	switch attr {
	case "id": fallthrough
	case "uri":
		return this.Id
	case "artist":
		return this.Artist
	case "title":
		return this.Title
	case "genre":
		return this.Genre
	case "album":
		return this.AlbumArtist
	case "albumartist":
		return this.AlbumArtist
	case "albumtrack":
		return this.AlbumTrack
	case "albumdisc":
		return this.AlbumDisc
	case "duration":
		return this.Duration
	}
	return nil
}

func (this *LocalTrack) GetArt() (image io.Reader) {
	this.player.withMpd(func(mpdc *mpd.Client) {
		numChunks := 0
		if strNum, err := mpdc.StickerGet(this.Id, "image-nchunks"); err == nil {
			if num, err := strconv.ParseInt(strNum, 10, 32); err == nil {
				numChunks = int(num)
			}
		}
		if numChunks == 0 {
			return
		}

		var chunks []io.Reader
		for i := 0; i < numChunks; i++ {
			if b64Data, err := mpdc.StickerGet(this.Id, fmt.Sprintf("image-%v", i)); err != nil {
				return
			} else {
				chunks = append(chunks, bytes.NewReader([]byte(b64Data)))
			}
		}
		image = base64.NewDecoder(base64.StdEncoding, io.MultiReader(chunks...))
	})
	return
}

func (this *LocalTrack) HasArt() (hasArt bool) {
	this.player.withMpd(func(mpdc *mpd.Client) {
		_, err := mpdc.StickerGet(this.Id, "image-nchunks")
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

	queuer *Queuer
}

func NewPlayer(mpdHost string, mpdPort int, mpdPassword *string, queuer *Queuer) (*Player, error) {
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

		addr: addr,
		passwd: passwd,

		queuer: queuer,
	}

	go player.idleLoop()
	go player.queuerEventsLoop()
	go player.queueLoop()
	go player.playlistLoop()

	return player, nil
}

func (this *Player) withMpd(fn func(mpd *mpd.Client)) {
	client, err := mpd.DialAuthenticated("tcp", this.addr, this.passwd)
	if err != nil {
		log.Println(err)
		this.withMpd(fn)
		return
	}
	defer client.Close()
	fn(client)
}

func (this *Player) idleLoop() {
	for {
		select {
		case event := <- this.mpdWatcher.Event:
			this.Emit(event)
		case err := <- this.mpdWatcher.Error:
			log.Println(err)
		}
	}
}

func (this *Player) queuerEventsLoop() {
	ch := make(chan string, 16)
	listenHandle := this.queuer.Listen(ch)
	defer close(ch)
	defer this.queuer.Unlisten(listenHandle)

	for {
		this.Emit("queuer-"+<-ch)
	}
}

func (this *Player) queueLoop() {
	listener := make(chan string, 16)
	this.Listen(listener)
	listener <- "player" // Bootstrap the cycle
	for {
		if event := <- listener; event != "player" {
			continue
		}

		this.withMpd(func(mpdc *mpd.Client) {
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
			track, _, err := this.CurrentTrack()
			if err != nil {
				log.Println(err)
				return
			}
			if track == nil {
				if err := this.QueueRandom(); err != nil {
					log.Println(err)
					return
				}
				if err = this.SetState("playing"); err != nil {
					log.Println(err)
					return
				}
			}
		})
	}
}

func (this *Player) playlistLoop() {
	listener := make(chan string, 16)
	this.Listen(listener)
	listener <- "playlist" // Bootstrap the cycle
	for {
		if event := <- listener; event != "playlist" {
			continue
		}

		this.withMpd(func(mpdc *mpd.Client) {
			songs, err := mpdc.PlaylistInfo(-1, -1)
			if err != nil {
				log.Println(err)
				return
			}

			// Synchronize queue attributes.
			// Remove tracks that are no longer in the list
			trackRemoveLoop: for id := range this.queueAttrs {
				for _, song := range songs {
					if song["file"] == id {
						continue trackRemoveLoop
					}
				}
				delete(this.queueAttrs, id)
			}

			// Initialize tracks that were wiped due to restarts or not added using
			// Trollibox.
			trackInitLoop: for _, song := range songs {
				for id := range this.queueAttrs {
					if song["file"] == id {
						continue trackInitLoop
					}
				}
				this.queueAttrs[song["file"]] = QueueAttrs{
					// Assume the track was queued by a human.
					QueuedBy: "user",
				}
			}

			if len(songs) > 0 {
				currentUri := songs[0]["file"]

				// TODO: If one track is followed by another track with the same
				// ID, the next block will not be executed, leaving the playcount
				// unchanged.
				if this.lastTrack != currentUri {
					// Streams can't have stickers.
					if !IsStreamUri(currentUri) {
						// Increment the playcount for this track.
						var playCount int64
						if str, err := mpdc.StickerGet(currentUri, "play-count"); err == nil {
							playCount, _ = strconv.ParseInt(str, 10, 32)
						}
						if err := mpdc.StickerSet(currentUri, "play-count", strconv.FormatInt(playCount + 1, 10)); err != nil {
							log.Printf("Could not set play-count: %v", err)
						}
					}

					this.lastTrack = currentUri
				}
			}
		})
	}
}

func (this *Player) localTrackFromMpdSong(song *mpd.Attrs, track *LocalTrack, mpdc *mpd.Client) {
	track.player = this

	if _, ok := (*song)["directory"]; ok {
		panic("Tried to read a directory as local file")
	}

	track.Id = (*song)["file"]
	if IsStreamUri(track.Id) {
		panic("Tried to read a stream as local file")
	}

	track.Artist      = (*song)["Artist"]
	track.Title       = (*song)["Title"]
	track.Genre       = (*song)["Genre"]
	track.Album       = (*song)["Album"]
	track.AlbumArtist = (*song)["AlbumArtist"]
	track.AlbumDisc   = (*song)["Disc"]
	track.AlbumTrack  = (*song)["Track"]

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

func (this *Player) streamTrackFromMpdSong(song *mpd.Attrs, stream *StreamTrack, mpdc *mpd.Client) {
	if tmpl := GetStreamByURL((*song)["file"]); tmpl != nil {
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

func (this *Player) playlistTrackFromMpdSong(song *mpd.Attrs, track *PlaylistTrack, mpdc *mpd.Client) {
	if IsStreamUri((*song)["file"]) {
		var streamTrack StreamTrack
		this.streamTrackFromMpdSong(song, &streamTrack, mpdc)
		track.Track = &streamTrack
	} else {
		var tr LocalTrack
		this.localTrackFromMpdSong(song, &tr, mpdc)
		track.Track = &tr
	}
	track.QueueAttrs = this.queueAttrs[track.Track.GetUri()]
}

func (this *Player) Queue(uri string, queuedBy string) (err error) {
	this.withMpd(func(mpdc *mpd.Client) {
		this.queueAttrs[uri] = QueueAttrs{
			QueuedBy: queuedBy,
		}
		err = mpdc.Add(uri)
	})
	return
}

func (this *Player) QueueRandom() error {
	tracks, err := this.ListTracks("", true)
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return nil
	}

	track := this.queuer.SelectTrack(tracks)
	if track == nil {
		log.Println("No tracks passed queue criteria")
		track = this.queuer.RandomTrack(tracks)
	}

	return this.Queue(track.GetUri(), "system")
}

func (this *Player) Volume() (vol float32, err error) {
	this.withMpd(func(mpdc *mpd.Client) {
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
			vol = this.lastVolume
		}
	})
	return
}

func (this *Player) SetVolume(vol float32) (err error) {
	this.withMpd(func(mpdc *mpd.Client) {
		if vol > 1 {
			vol = 1
		} else if vol < 0 {
			vol = 0
		}

		this.lastVolume = vol
		err = mpdc.SetVolume(int(vol * 100))
	})
	return
}

func (this *Player) ListTracks(path string, recursive bool) (tracks []LocalTrack, err error) {
	this.withMpd(func(mpdc *mpd.Client) {
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
				this.localTrackFromMpdSong(&song, &tracks[i-numDirs], mpdc)
			}
		}
		tracks = tracks[:len(tracks)-numDirs]
	})
	return
}

func (this *Player) Playlist() (tracks []PlaylistTrack, err error) {
	this.withMpd(func(mpdc *mpd.Client) {
		var songs []mpd.Attrs
		songs, err = mpdc.PlaylistInfo(-1, -1)
		if err != nil {
			return
		}

		tracks = make([]PlaylistTrack, len(songs))
		for i, song := range songs {
			this.playlistTrackFromMpdSong(&song, &tracks[i], mpdc)
		}
	})
	return
}

func (this *Player) SetPlaylistIds(trackIds []string) error {
	playlist, err := this.Playlist()
	if err != nil {
		return err
	}

	this.withMpd(func(mpd *mpd.Client) {
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
func (this *Player) CurrentTrack() (track *PlaylistTrack, progress int, err error) {
	this.withMpd(func(mpdc *mpd.Client) {
		var status mpd.Attrs
		status, err = mpdc.Status()
		if err != nil {
			return
		}

		if st, ok := status["state"]; !ok || st == "stop" {
			return
		}

		var playlist []PlaylistTrack
		playlist, err = this.Playlist()
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

func (this *Player) SetProgress(progress int) (err error) {
	this.withMpd(func(mpdc *mpd.Client) {
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

func (this *Player) Next() (err error) {
	this.withMpd(func(mpdc *mpd.Client) {
		if err = mpdc.Next(); err != nil {
			return
		}
		err = mpdc.Delete(0, 1)
	})
	return
}

func (this *Player) State() (state string, err error) {
	this.withMpd(func(mpdc *mpd.Client) {
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

func (this *Player) SetState(state string) (err error) {
	this.withMpd(func(mpdc *mpd.Client) {
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

func (this *Player) Queuer() *Queuer {
	return this.queuer
}
