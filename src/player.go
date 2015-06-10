package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
	mpd "github.com/jteeuwen/go-pkg-mpd"
)

type Track struct {
	Id       string  `json:"id"`
	Artist   string  `json:"artist"`
	Title    string  `json:"title"`
	Album    string  `json:"album"`
	Art      *string `json:"art"`
	Duration int     `json:"duration"`
}

func TrackFromMpdSong(song *mpd.Song, track *Track) {
	track.Id       = song.File
	track.Artist   = song.Artist
	track.Title    = song.Title
	track.Album    = song.Album
	track.Art      = nil
	track.Duration = song.Time
}


type PlaylistTrack struct {
	Track
	AddedBy string `json:"addedby"`
}

func PlaylistTrackFromMpdSong(song *mpd.Song, track *PlaylistTrack) {
	TrackFromMpdSong(song, &track.Track)
	track.AddedBy = "robot" // TODO: Store and look this up
}


type Player struct {
	rand *rand.Rand
	mpd  *mpd.Client

	// Running the idle routine on the same connection as the main connection
	// will fuck things up badly.
	mpdIdle *mpd.Client

	// Commands may be coming in concurrently. We have to make sure that
	// only one calling routine has exclusive access.
	mpdLock sync.Mutex

	listeners     map[uint64]chan string
	listenersEnum uint64
	listenersLock sync.Mutex
}

func NewPlayer(mpdHost string, mpdPort int, mpdPassword *string) (*Player, error) {
	var passwd string
	if mpdPassword != nil {
		passwd = *mpdPassword
	} else {
		passwd = ""
	}

	client, err := mpd.Dial(fmt.Sprintf("%v:%v", mpdHost, mpdPort), passwd)
	if err != nil {
		return nil, err
	}

	clientIdle, err := mpd.Dial(fmt.Sprintf("%v:%v", mpdHost, mpdPort), passwd)
	if err != nil {
		return nil, err
	}

	player := &Player{
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
		mpd:       client,
		mpdIdle:   clientIdle,
		listeners: map[uint64]chan string{},
	}

	queueListener := make(chan string, 16)
	go player.queueLoop(queueListener)
	player.Listen(queueListener)
	queueListener <- "player" // Bootstrap the cycle

	go player.pingLoop()
	go player.idleLoop()

	return player, nil
}

func (this *Player) pingLoop() {
	for {
		this.mpdLock.Lock()

		var buf [3]byte // ok + \n
		this.mpd.Write([]byte("ping\n"))
		this.mpd.Read(buf[:])

		this.mpdLock.Unlock()
		time.Sleep(20 * time.Second)
	}
}

func (this *Player) idleLoop() {
	var eventNames = map[mpd.SubSystem]string{
		mpd.DatabaseSystem:       "database",
		mpd.MessageSystem:        "message",
		mpd.MixerSystem:          "volume",
		mpd.OutputSystem:         "output",
		mpd.PlayerSystem:         "player",
		mpd.PlaylistSystem:       "playlist",
		mpd.StickerSystem:        "sticker",
		mpd.StoredPlaylistSystem: "stored-playlist",
		mpd.SubscriptionSystem:   "subscription",
		mpd.UpdateSystem:         "update",
	}

	for {
		sub, err := this.mpdIdle.Idle()
		if err != nil {
			log.Println(err)
			continue
		}

		this.listenersLock.Lock()
		for _, l := range this.listeners {
			l <- eventNames[sub]
		}
		this.listenersLock.Unlock()
	}
}

func (this *Player) queueLoop(listener chan string) {
	for {
		if event := <- listener; event == "player" {
			if track, _, err := this.CurrentTrack(); err != nil {
				fmt.Println(err)
				continue
			} else if track == nil {
				this.mpdLock.Lock()
				err := this.mpd.Clear()
				this.mpdLock.Unlock()
				if err != nil {
					log.Println(err)
					continue
				}
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

func (this *Player) Queue(path string) error {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	log.Printf("Queueing \"%v\"", path)
	return this.mpd.Add(path)
}

func (this *Player) QueueRandom() error {
	files, err := this.ListTracks("")
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	// TODO: Implement selection bias
	// TODO: Directories should not be used
	return this.Queue(files[this.rand.Intn(len(files))].Id)
}

func (this *Player) Volume() (float32, error) {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	status, err := this.mpd.Status()
	if err != nil {
		return 0, err
	}
	// Here, the volume scale is form 0 to 255...
	return float32(status.Volume) / 255, nil
}

func (this *Player) SetVolume(vol float32) error {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	if vol > 1 {
		vol = 1
	} else if vol < 0 {
		vol = 0
	}

	// ...And here, the volume scale is form 0 to 100!
	if err := this.mpd.Volume(byte(vol * 100), false); err != nil {
		return err
	}
	return nil
}

func (this *Player) ListTracks(path string) ([]Track, error) {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	songs, err := this.mpd.ListInfo(path)
	if err != nil {
		return nil, err
	}

	tracks := make([]Track, len(songs))
	for i, song := range songs {
		TrackFromMpdSong(song, &tracks[i])
	}
	return tracks, nil
}

func (this *Player) Playlist() ([]PlaylistTrack, error) {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	songs, err := this.mpd.PlaylistInfo(-1)
	if err != nil {
		return nil, err
	}

	tracks := make([]PlaylistTrack, len(songs))
	for i, song := range songs {
		PlaylistTrackFromMpdSong(song, &tracks[i])
	}
	return tracks, nil
}

// Returns the currently playing track as well as its progress in seconds
func (this *Player) CurrentTrack() (*Track, int, error) {
	this.mpdLock.Lock()
	args, err := this.mpd.Current()
	this.mpdLock.Unlock()
	if err != nil {
		return nil, 0, err
	}

	var current []Track
	if f := args.S("file"); f != "" {
		current, err = this.ListTracks(f)
		if err != nil {
			return nil, 0, err
		}
	}

	if len(current) > 0 {
		this.mpdLock.Lock()
		status, err := this.mpd.Status()
		this.mpdLock.Unlock()
		if err != nil {
			return nil, 0, err
		}
		return &current[0], int(status.Elapsed), err

	} else {
		return nil, 0, nil
	}
}

func (this *Player) Next() error {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	err := this.mpd.Delete(0)
	if err != nil {
		return err
	}
	return this.mpd.Next()
}

func (this *Player) State() (string, error) {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	status, err := this.mpd.Status()
	if err != nil {
		return "", err
	}

	return map[mpd.PlayState]string{
		mpd.Paused:  "paused",
		mpd.Playing: "playing",
		mpd.Stopped: "stopped",
	}[status.State], nil
}

func (this *Player) SetState(state string) (error) {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	switch state {
	case "paused":
		return this.mpd.Pause(true)
	case "playing":
		if status, err := this.mpd.Status(); err != nil {
			return err
		} else if status.State == mpd.Stopped {
			this.mpd.Play(0)
		} else {
			return this.mpd.Pause(false)
		}
	case "stopped":
		return this.mpd.Stop()
	default:
		return fmt.Errorf("Unknown play state %v", state)
	}

	return nil
}
