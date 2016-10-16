package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"../../player"
	raw "../raw"
)

type Server struct {
	rawServer *raw.Server
}

func NewServer(rawServer *raw.Server) (*Server, error) {
	if _, err := exec.LookPath("youtube-dl"); err != nil {
		return nil, fmt.Errorf("Youtube server not available: %v", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("Youtube server not available: %v", err)
	}
	return &Server{
		rawServer: rawServer,
	}, nil
}

func (sv *Server) Download(url string) (player.Track, error) {
	ctx, cancel := context.WithCancel(context.Background())

	info, err := readMediaInfo(ctx, url)
	if err != nil {
		cancel()
		return player.Track{}, err
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
		"-acodec", "libvorbis",
		"-f", "ogg",
		"-",
	)
	conversion.Stdin, _ = download.StdoutPipe()
	convOut, _ := conversion.StdoutPipe()

	if err := download.Start(); err != nil {
		cancel()
		return player.Track{}, err
	}
	go download.Wait()
	if err := conversion.Start(); err != nil {
		cancel()
		return player.Track{}, err
	}
	go conversion.Wait()

	track, err := sv.rawServer.Add(convOut, info.Title)
	if err != nil {
		cancel()
		return player.Track{}, err
	}
	return track, nil
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
