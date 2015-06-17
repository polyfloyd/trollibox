package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"
	mpd "github.com/polyfloyd/gompd/mpd"
)


type Track struct {
	player *Player
	isDir  bool

	Id       string  `json:"id"`
	Artist   string  `json:"artist"`
	Title    string  `json:"title"`
	Genre    string  `json:"genre"`
	Album    string  `json:"album"`
	Art      *string `json:"art"`
	Duration int     `json:"duration"`
}

func (this *Track) GetArt() (image io.Reader) {
	this.player.withMpd(func(mpdc *mpd.Client) {
		numChunks := 0
		if strNum, err := this.player.mpd.StickerGet(this.Id, "image-nchunks"); err == nil {
			if num, err := strconv.ParseInt(strNum, 10, 32); err == nil {
				numChunks = int(num)
			}
		}
		if numChunks == 0 {
			return
		}

		var chunks []io.Reader
		for i := 0; i < numChunks; i++ {
			if b64Data, err := this.player.mpd.StickerGet(this.Id, fmt.Sprintf("image-%v", i)); err != nil {
				return
			} else {
				chunks = append(chunks, bytes.NewReader([]byte(b64Data)))
			}
		}
		image = base64.NewDecoder(base64.StdEncoding, io.MultiReader(chunks...))
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
	rand *rand.Rand
	mpd  *mpd.Client

	// Running the idle routine on the same connection as the main connection
	// will fuck things up badly.
	mpdWatcher *mpd.Watcher

	// Commands may be coming in concurrently. We have to make sure that
	// only one calling routine has exclusive access.
	mpdLock sync.Mutex

	addr, passwd string

	listeners     map[uint64]chan string
	listenersEnum uint64
	listenersLock sync.Mutex

	// A map containing properties related to tracks currently in the queue.
	queueAttrs map[string]QueueAttrs

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

	client, err := mpd.DialAuthenticated("tcp", addr, passwd)
	if err != nil {
		return nil, err
	}

	clientWatcher, err := mpd.NewWatcher("tcp", addr, passwd)
	if err != nil {
		return nil, err
	}

	player := &Player{
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
		mpd:        client,
		mpdWatcher: clientWatcher,
		listeners:  map[uint64]chan string{},
		queueAttrs: map[string]QueueAttrs{},

		addr:   addr,
		passwd: passwd,
	}

	go player.queueLoop()
	go player.playlistLoop()
	go player.pingLoop()
	go player.idleLoop()

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

func (this *Player) pingLoop() {
	for {
		this.mpdLock.Lock()
		this.mpd.Ping()
		this.mpdLock.Unlock()
		time.Sleep(20 * time.Second)
	}
}

func (this *Player) idleLoop() {
	for {
		select {
		case event := <- this.mpdWatcher.Event:
			this.listenersLock.Lock()
			for _, l := range this.listeners {
				l <- event
			}
			this.listenersLock.Unlock()
		case err := <- this.mpdWatcher.Error:
			log.Println(err)
		}
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

		// Remove played tracks form the queue.
		this.mpdLock.Lock()
		status, err := this.mpd.Status()
		if err != nil {
			this.mpdLock.Unlock()
			log.Println(err)
			continue
		}
		songIndex := 0
		if str, ok := status["song"]; ok {
			if song64, err := strconv.ParseInt(str, 10, 32); err == nil {
				songIndex = int(song64)
			} else {
				log.Println(err)
			}
		} else if status["state"] == "stop" {
			// Quick fix to make sure the previous track is not played twice.
			// TODO: Some funny stuff happens when MPD receives a stop command.
			songIndex = 1
		}
		if songIndex != 0 {
			if err := this.mpd.Delete(0, songIndex); err != nil {
				log.Println(err)
			}
		}
		this.mpdLock.Unlock()

		// Queue a new track if nothing is playing.
		track, _, err := this.CurrentTrack()
		if err != nil {
			log.Println(err)
			continue
		}
		if track == nil {
			if err := this.QueueRandom(); err != nil {
				log.Println(err)
				continue
			}
			if err = this.SetState("playing"); err != nil {
				log.Println(err)
				continue
			}
		}
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

		this.mpdLock.Lock()

		songs, err := this.mpd.PlaylistInfo(-1, -1)
		if err != nil {
			this.mpdLock.Unlock()
			log.Println(err)
			continue
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
			var current Track
			this.trackFromMpdSong(&songs[0], &current, this.mpd)
			// TODO: If one track is followed by another track with the same
			// ID, the next block will not be executed, leaving the playcount
			// unchanged.
			if this.lastTrack != current.Id {

				// Increment the playcount for this track
				var playCount int64
				if str, err := this.mpd.StickerGet(current.Id, "play-count"); err == nil {
					playCount, _ = strconv.ParseInt(str, 10, 32)
				}
				if err := this.mpd.StickerSet(current.Id, "play-count", strconv.FormatInt(playCount + 1, 10)); err != nil {
					log.Println(err)
				}

				this.lastTrack = current.Id
			}
		}

		this.mpdLock.Unlock()
	}
}

func (this *Player) trackFromMpdSong(song *mpd.Attrs, track *Track, mpdc *mpd.Client) {
	track.player = this

	if dir, ok := (*song)["directory"]; ok {
		track.isDir = true
		track.Id = dir
	} else {
		track.isDir = false
		track.Id = (*song)["file"]
	}

	track.Artist = (*song)["Artist"]
	track.Title  = (*song)["Title"]
	track.Genre  = (*song)["Genre"]
	track.Album  = (*song)["Album"]
	track.Art    = nil

	// Who the fuck thought it was a good idea to mix capitals and lowercase
	// for the time?!
	var timeStr string
	if str, ok := (*song)["Time"]; ok {
		timeStr = str
	} else if str, ok := (*song)["time"]; ok {
		timeStr = str
	}

	if duration, err := strconv.ParseInt(timeStr, 10, 32); err != nil {
		panic(err)
	} else {
		track.Duration = int(duration)
	}

	if _, err := mpdc.StickerGet(track.Id, "image-nchunks"); err == nil {
		url := "/data/track/art/"+track.Id
		track.Art = &url
	}
}

func (this *Player) playlistTrackFromMpdSong(song *mpd.Attrs, track *PlaylistTrack, mpdc *mpd.Client) {
	this.trackFromMpdSong(song, &track.Track, mpdc)
	track.QueueAttrs = this.queueAttrs[track.Id]
}

func (this *Player) Listen(listener chan string) uint64 {
	this.listenersLock.Lock()
	defer this.listenersLock.Unlock()

	this.listenersEnum++
	this.listeners[this.listenersEnum] = listener
	return this.listenersEnum
}

func (this *Player) Unlisten(handle uint64) {
	this.listenersLock.Lock()
	defer this.listenersLock.Unlock()

	delete(this.listeners, handle)
}

func (this *Player) Queue(path string, queuedBy string) (err error) {
	this.withMpd(func(mpdc *mpd.Client) {
		this.queueAttrs[path] = QueueAttrs{
			QueuedBy: queuedBy,
		}
		err = mpdc.Add(path)
	})
	return
}

func (this *Player) QueueRandom() error {
	files, err := this.ListTracks("", true)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	tracks := make([]Track, len(files))[0:0]
	for _, file := range files {
		if !file.isDir {
			tracks = append(tracks, file)
		}
	}

	// TODO: Implement selection bias
	return this.Queue(tracks[this.rand.Intn(len(tracks))].Id, "system")
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

func (this *Player) ListTracks(path string, recursive bool) (tracks []Track, err error) {
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

		tracks = make([]Track, len(songs))
		for i, song := range songs {
			this.trackFromMpdSong(&song, &tracks[i], mpdc)
		}
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
		if playlist[0].Id == trackIds[0] {
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
		if err != nil {
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
