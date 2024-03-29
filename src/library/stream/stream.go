package stream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"

	"trollibox/src/library"
	"trollibox/src/util"
)

var ErrInvalidArgument = errors.New("invalid argument")

var dataURIRe = regexp.MustCompile("^data:([a-z]+/[a-z]+);base64,(.+)$")
var m3uTemplate = template.Must(template.New("m3u").Parse(
	`#EXTM3U

{{ with .ArtURI }}#EXTART:{{ . }}{{ end }}
#EXTINF:1,{{ .Title }}
{{ .URL }}
`))

// A Stream represents audio that is streamed over HTTP.
type Stream struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	ArtURI   string `json:"arturi,omitempty"`
	modTime  time.Time
}

func loadM3U(filename string) (*Stream, error) {
	fd, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error loading stream from M3U: %v", err)
	}
	defer fd.Close()

	stat, err := fd.Stat()
	if err != nil {
		return nil, err
	}

	stream := &Stream{
		Filename: path.Base(filename),
		modTime:  stat.ModTime(),
	}

	m3u := bufio.NewReader(fd)
	// The first line should be the M3U header: #EXTM3U
	firstLine, err := m3u.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if firstLine != "#EXTM3U\n" {
		return nil, fmt.Errorf("error loading stream from M3U: expected \"#EXTM3U\" as first line, got %q", firstLine)
	}

	for {
		lineStart, err := m3u.Peek(7)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("error loading stream from M3U: %v", err)
		}

		if string(lineStart) == "#EXTART" {
			if _, err := m3u.Discard(len("#EXTART:")); err != nil {
				return nil, fmt.Errorf("error loading stream from M3U: %v", err)
			}
			art, err := m3u.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("error loading stream from M3U: %v", err)
			}
			stream.ArtURI = art[0 : len(art)-1]

		} else if string(lineStart) == "#EXTINF" {
			if _, err := m3u.ReadString(','); err != nil {
				return nil, fmt.Errorf("error loading stream from M3U: %v", err)
			}
			title, err := m3u.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("error loading stream from M3U: %v", err)
			}
			stream.Title = title[0 : len(title)-1]

		} else if string(lineStart[0:4]) == "http" {
			url, err := m3u.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("error loading stream from M3U: %v", err)
			}
			stream.URL = url[0 : len(url)-1]

		} else {
			if _, err := m3u.Discard(1); err == io.EOF {
				break
			} else if err != nil {
				return nil, fmt.Errorf("error loading stream from M3U: %v", err)
			}
		}
	}

	if stream.URL == "" {
		return nil, fmt.Errorf("error loading stream from M3U: Empty URL")
	}
	return stream, nil
}

func (stream *Stream) encodeM3U(out io.Writer) error {
	return m3uTemplate.Execute(out, stream)
}

// PlayerTrack builds a library track for use in players.
func (stream *Stream) PlayerTrack() library.Track {
	return library.Track{
		URI:   stream.URL,
		Title: stream.Title,
	}
}

func (stream *Stream) art() (*library.Art, error) {
	if stream.ArtURI == "" {
		return nil, library.ErrNoArt
	}
	match := dataURIRe.FindStringSubmatch(stream.ArtURI)
	if len(match) == 0 {
		return nil, fmt.Errorf("stream %v: malformed stream art", stream.Title)
	}
	imageData, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(match[2])))
	if err != nil {
		return nil, err
	}
	return &library.Art{
		ImageData: imageData,
		MimeType:  match[1],
		ModTime:   stream.modTime,
	}, nil
}

func (stream *Stream) String() string {
	if stream.Title != "" {
		return fmt.Sprintf("Stream{%s, %q}", stream.URL, stream.Title)
	}
	return fmt.Sprintf("Stream{%s}", stream.URL)
}

// DB is a database that handles persistent storage of a collection of streams.
type DB struct {
	util.Emitter

	directory string
}

// NewDB creates a new stream database that stores streams in the specified directory.
//
// The directory is recursively created if it does not exists. An error is
// returned if directory creation fails.
func NewDB(directory string) (*DB, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	return &DB{directory: directory}, nil
}

// Streams returns a list of all streams stored.
func (db *DB) Streams() ([]Stream, error) {
	fd, err := os.Open(db.directory)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	files, err := fd.Readdir(0)
	if err != nil {
		return nil, err
	}
	streams := make([]Stream, 0, len(files))
	for _, file := range files {
		if path.Ext(file.Name()) == ".m3u" {
			stream, err := db.StreamByFilename(file.Name())
			if err != nil {
				slog.Error("Unable to load stream", "path", file.Name(), "error", err)
				continue
			}
			streams = append(streams, *stream)
		}
	}
	return streams, nil
}

// StreamByFilename looks up a stream by it's filename including extension.
func (db *DB) StreamByFilename(filename string) (*Stream, error) {
	return loadM3U(path.Join(db.directory, path.Clean(filename)))
}

// RemoveStream removes a stream from the database.
//
// This is a no-op if the specified stream does not exists.
func (db *DB) RemoveStream(stream *Stream) error {
	if path.Ext(stream.Filename) != ".m3u" {
		return fmt.Errorf("stream filenames must have the .m3u suffix")
	}
	err := os.Remove(path.Join(db.directory, path.Clean(stream.Filename)))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	db.Emit(library.UpdateEvent{})
	return nil
}

// StoreStream stores the specified stream using its filename.
//
// If no filename is given, it will be inferred from the URL.
//
// An error is returned if the filename is already set and does not have "m3u"
// as extension.
//
// If the stream specifies remote artwork, it is downloaded. An error is
// returned if downloading fails or something other than an image was
// downloaded.
func (db *DB) StoreStream(stream *Stream) error {
	if stream.Title == "" {
		return fmt.Errorf("%w: title not set", ErrInvalidArgument)
	}
	if stream.URL == "" {
		return fmt.Errorf("%w: url not set", ErrInvalidArgument)
	} else if !regexp.MustCompile(`^https?://`).MatchString(stream.URL) {
		return fmt.Errorf("%w: invalid URL", ErrInvalidArgument)
	} else if regexp.MustCompile(`^https?:\/\/(?:www\.)youtube\.com`).MatchString(stream.URL) {
		return fmt.Errorf("%w: that is definitely not an internet radio URL", ErrInvalidArgument)
	}

	if stream.Filename == "" {
		stream.Filename = filenameFromURL(stream.URL) + ".m3u"
	}
	if path.Ext(stream.Filename) != ".m3u" {
		return fmt.Errorf("%w: stream filenames must have the .m3u suffix", ErrInvalidArgument)
	}

	// Download the track art and store it as a data URI.
	if stream.ArtURI != "" && !dataURIRe.MatchString(stream.ArtURI) {
		artURI, err := downloadToDataURI(stream.ArtURI)
		if err != nil {
			return err
		}
		stream.ArtURI = artURI
	}

	fd, err := os.Create(path.Join(db.directory, path.Clean(stream.Filename)))
	if err != nil {
		return err
	}
	if err := stream.encodeM3U(fd); err != nil {
		return err
	}
	db.Emit(library.UpdateEvent{})
	return nil
}

// Tracks implements the library.Library interface.
func (db *DB) Tracks(ctx context.Context) ([]library.Track, error) {
	streams, err := db.Streams()
	if err != nil {
		return nil, err
	}
	tracks := make([]library.Track, len(streams))
	for i, stream := range streams {
		tracks[i] = stream.PlayerTrack()
	}
	return tracks, nil
}

// TrackInfo implements the library.Library interface.
func (db *DB) TrackInfo(ctx context.Context, uris ...string) ([]library.Track, error) {
	tracks := make([]library.Track, len(uris))
	streams, err := db.Streams()
	if err != nil {
		return nil, err
	}
	for i, uri := range uris {
		for _, stream := range streams {
			if stream.URL == uri {
				tracks[i] = stream.PlayerTrack()
			}
		}
	}
	return tracks, nil
}

// TrackArt implements the library.Library interface.
func (db *DB) TrackArt(ctx context.Context, track string) (*library.Art, error) {
	if !strings.HasPrefix(track, "http") {
		return nil, fmt.Errorf("%w: not a stream URL", library.ErrNoArt)
	}
	stream, err := db.streamByURL(track)
	if stream == nil {
		return nil, fmt.Errorf("%w: no such stream", library.ErrNoArt)
	}
	if err != nil {
		return nil, err
	}
	return stream.art()
}

// Events implements the player.Player interface.
func (db *DB) Events() *util.Emitter {
	return &db.Emitter
}

func (db *DB) String() string {
	return fmt.Sprintf("StreamDB{%s}", db.directory)
}

func (db *DB) streamByURL(url string) (*Stream, error) {
	streams, err := db.Streams()
	if err != nil {
		return nil, err
	}
	for _, stream := range streams {
		if stream.URL == url {
			return &stream, nil
		}
	}
	return nil, nil
}

func filenameFromURL(url string) string {
	return regexp.MustCompile(`\W`).ReplaceAllString(url, "_")
}

func downloadToDataURI(url string) (string, error) {
	client := http.Client{Timeout: time.Second * 30}
	res, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not fetch image: %v", err)
	}
	defer res.Body.Close()

	contentType := res.Header.Get("Content-Type")
	if !regexp.MustCompile("^image/").MatchString(contentType) {
		return "", fmt.Errorf("%w: invalid content type for image: %s", ErrInvalidArgument, contentType)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "data:%s;base64,", contentType)
	if _, err := io.Copy(base64.NewEncoder(base64.StdEncoding, &buf), res.Body); err != nil {
		return "", fmt.Errorf("could not fetch image: %v", err)
	}
	return buf.String(), nil
}
