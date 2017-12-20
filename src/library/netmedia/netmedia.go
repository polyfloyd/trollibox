package netmedia

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/library/raw"
	"github.com/polyfloyd/trollibox/src/util"
)

// A Server is able to fetch audio files from various websites and expose them
// using a raw.Server.
type Server struct {
	rawServer *raw.Server
}

// NewServer creates a new Server using the specified raw server as backend.
func NewServer(rawServer *raw.Server) (*Server, error) {
	if _, err := exec.LookPath("youtube-dl"); err != nil {
		return nil, fmt.Errorf("Netmedia server not available: %v", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("Netmedia server not available: %v", err)
	}
	return &Server{
		rawServer: rawServer,
	}, nil
}

// Download attempts to retrieve an audio file from the specified URL and
// returns a track that, when added to player's queue plays the downloaded
// file.
//
// The returned track's audio stream may be incomplete as downloading happens
// in the background.
func (sv *Server) Download(url string) (library.Track, <-chan error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	info, err := readMediaInfo(ctx, url)
	if err != nil {
		return library.Track{}, util.ErrorAsChannel(err)
	}

	download := exec.CommandContext(ctx,
		"youtube-dl",
		url,
		"--output", "-",
	)
	conversion := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", "-",
		"-vn",
		"-acodec", "libmp3lame",
		"-f", "mp3",
		"-",
	)
	conversion.Stdin, _ = download.StdoutPipe()
	convOut, _ := conversion.StdoutPipe()

	if err := download.Start(); err != nil {
		return library.Track{}, util.ErrorAsChannel(err)
	}
	go download.Wait()
	if err := conversion.Start(); err != nil {
		return library.Track{}, util.ErrorAsChannel(err)
	}
	go conversion.Wait()

	var image []byte
	var imageMime string
	if info.Thumbnail != "" {
		if resp, err := http.Get(info.Thumbnail); err == nil {
			defer resp.Body.Close()
			image, _ = ioutil.ReadAll(resp.Body)
			imageMime = resp.Header.Get("Content-Type")
		}
	}
	return sv.rawServer.Add(convOut, info.Title, image, imageMime)
}

// RawServer returns the underlying raw.Server.
func (sv *Server) RawServer() *raw.Server {
	return sv.rawServer
}

type mediaInfo struct {
	Thumbnail string `json:"thumbnail"`
	Title     string `json:"title"`
}

func readMediaInfo(ctx context.Context, url string) (mediaInfo, error) {
	infoJSON, err := exec.CommandContext(ctx, "youtube-dl", url, "--dump-json").Output()
	if err != nil {
		return mediaInfo{}, err
	}
	var info mediaInfo
	err = json.Unmarshal(infoJSON, &info)
	return info, err
}
