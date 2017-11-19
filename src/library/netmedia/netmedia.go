package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"

	"../../player"
	"../../util"
	raw "../raw"
)

type Server struct {
	rawServer *raw.Server
}

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

func (sv *Server) Download(url string) (player.Track, <-chan error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	info, err := readMediaInfo(ctx, url)
	if err != nil {
		return player.Track{}, util.ErrorAsChannel(err)
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
		return player.Track{}, util.ErrorAsChannel(err)
	}
	go download.Wait()
	if err := conversion.Start(); err != nil {
		return player.Track{}, util.ErrorAsChannel(err)
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

func (sv *Server) RawServer() *raw.Server {
	return sv.rawServer
}

type mediaInfo struct {
	Thumbnail string `json:"thumbnail"`
	Title     string `json:"title"`
}

func readMediaInfo(ctx context.Context, url string) (mediaInfo, error) {
	infoJson, err := exec.CommandContext(ctx, "youtube-dl", url, "--dump-json").Output()
	if err != nil {
		return mediaInfo{}, err
	}
	var info mediaInfo
	err = json.Unmarshal(infoJson, &info)
	return info, err
}
