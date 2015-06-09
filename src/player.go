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


type Player struct {
	rand *rand.Rand
	mpd  *mpd.Client

	// Running the idle routine on the same connection as the main connection
	// will fuck things up badly.
	mpdIdle *mpd.Client

	// Commands may be coming in concurrently. We have to make sure that
	// only one calling routine has exclusive access.
	mpdLock sync.Mutex
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
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
		mpd:     client,
		mpdIdle: clientIdle,
	}

	go player.pingloop()
	go player.idleloop()

	player.mpdLock.Lock()
	if err := player.mpd.Clear(); err != nil {
		player.mpdLock.Unlock()
		return nil, err
	}
	player.mpdLock.Unlock()

	if err := player.QueueRandom(); err != nil {
		return nil, err
	}

	player.mpdLock.Lock()
	if err := player.mpd.Play(0); err != nil {
		player.mpdLock.Unlock()
		return nil, err
	}
	player.mpdLock.Unlock()

	if err := player.Play(); err != nil {
		return nil, err
	}

	return player, nil
}

func (this *Player) pingloop() {
	for {
		this.mpdLock.Lock()

		var buf [3]byte // ok + \n
		this.mpd.Write([]byte("ping\n"))
		this.mpd.Read(buf[:])

		this.mpdLock.Unlock()
		time.Sleep(20 * time.Second)
	}
}

func (this *Player) idleloop() {
	for {
		sub, err := this.mpdIdle.Idle()
		if err != nil {
			log.Println(err)
			continue
		}

		// TODO: fire events
		_ = sub
	}
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

func (this *Player) Play() error {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	return this.mpd.Pause(false)
}

func (this *Player) Pause() error {
	this.mpdLock.Lock()
	defer this.mpdLock.Unlock()

	return this.mpd.Pause(true)
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
