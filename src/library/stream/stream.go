package stream

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/polyfloyd/trollibox/src/player"
	"github.com/polyfloyd/trollibox/src/util"
)

var dataUriRe = regexp.MustCompile("^data:([a-z]+/[a-z]+);base64,(.+)$")
var m3uTemplate = template.Must(template.New("m3u").Parse(
	`#EXTM3U

{{ with .ArtURI }}#EXTART:{{ . }}{{ end }}
#EXTINF:1,{{ .Title }}
{{ .URL }}
`))

type Stream struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	ArtURI   string `json:"arturi"`
}

func LoadM3U(filename string) (*Stream, error) {
	fd, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
	}
	defer fd.Close()

	stream := &Stream{Filename: path.Base(filename)}

	m3u := bufio.NewReader(fd)
	// The first line should be the M3U header: #EXTM3U
	firstLine, err := m3u.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if firstLine != "#EXTM3U\n" {
		return nil, fmt.Errorf("Error loading stream from M3U: first line is not \"#EXTM3U\"")
	}

	for {
		lineStart, err := m3u.Peek(7)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
		}

		if string(lineStart) == "#EXTART" {
			m3u.Discard(len("#EXTART:"))
			art, err := m3u.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
			}
			stream.ArtURI = art[0 : len(art)-1]

		} else if string(lineStart) == "#EXTINF" {
			m3u.ReadString(',')
			title, err := m3u.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
			}
			stream.Title = title[0 : len(title)-1]

		} else if string(lineStart[0:4]) == "http" {
			url, err := m3u.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
			}
			stream.URL = url[0 : len(url)-1]

		} else {
			if _, err := m3u.Discard(1); err == io.EOF {
				break
			} else if err != nil {
				return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
			}
		}
	}

	if stream.URL == "" {
		return nil, fmt.Errorf("Error loading stream from M3U: Empty URL")
	}
	return stream, nil
}

func (stream *Stream) EncodeM3U(out io.Writer) error {
	return m3uTemplate.Execute(out, stream)
}

func (stream *Stream) PlayerTrack() player.Track {
	return player.Track{
		Uri:    stream.URL,
		Title:  stream.Title,
		HasArt: stream.ArtURI != "",
	}
}

func (stream *Stream) Art() (io.ReadCloser, string) {
	if match := dataUriRe.FindStringSubmatch(stream.ArtURI); len(match) > 0 {
		return ioutil.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(match[2]))), match[1]
	}
	return nil, ""
}

func (stream *Stream) String() string {
	if stream.Title != "" {
		return fmt.Sprintf("Stream{%s, %q}", stream.URL, stream.Title)
	}
	return fmt.Sprintf("Stream{%s}", stream.URL)
}

type DB struct {
	util.Emitter

	directory string
}

func NewDB(directory string) (*DB, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	return &DB{directory: directory}, nil
}

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
			if stream, err := db.StreamByFilename(file.Name()); err == nil {
				streams = append(streams, *stream)
			} else {
				log.Printf("Unable to load stream from %q: %v", file.Name(), err)
			}
		}
	}
	return streams, nil
}

func (db *DB) StreamByFilename(filename string) (*Stream, error) {
	return LoadM3U(path.Join(db.directory, path.Clean(filename)))
}

func (db *DB) RemoveStream(stream *Stream) error {
	if path.Ext(stream.Filename) != ".m3u" {
		return fmt.Errorf("Stream filenames must have the .m3u suffix")
	}
	defer db.Emit("update")
	return os.Remove(path.Join(db.directory, path.Clean(stream.Filename)))
}

func (db *DB) StoreStream(stream *Stream) error {
	if stream.Filename == "" {
		stream.Filename = filenameFromURL(stream.URL) + ".m3u"
	}
	if path.Ext(stream.Filename) != ".m3u" {
		return fmt.Errorf("Stream filenames must have the .m3u suffix")
	}

	// Download the track art and store it as a data URI.
	if stream.ArtURI != "" && !dataUriRe.MatchString(stream.ArtURI) {
		artURI, contentType, err := downloadToDataUri(stream.ArtURI)
		if err != nil {
			return err
		}
		if !regexp.MustCompile("^image/").MatchString(contentType) {
			return fmt.Errorf("Invalid content type for stream image: %s", contentType)
		}
		stream.ArtURI = artURI
	}

	fd, err := os.Create(path.Join(db.directory, path.Clean(stream.Filename)))
	if err != nil {
		return err
	}
	defer db.Emit("update")
	return stream.EncodeM3U(fd)
}

func (db *DB) Tracks() ([]player.Track, error) {
	streams, err := db.Streams()
	if err != nil {
		return nil, err
	}
	tracks := make([]player.Track, len(streams))
	for i, stream := range streams {
		tracks[i] = stream.PlayerTrack()
	}
	return tracks, nil
}

func (db *DB) TrackInfo(uris ...string) ([]player.Track, error) {
	tracks := make([]player.Track, len(uris))
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

func (db *DB) TrackArt(track string) (image io.ReadCloser, mime string) {
	stream, err := db.streamByURL(track)
	if stream == nil || err != nil {
		return nil, ""
	}
	return stream.Art()
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
	return regexp.MustCompile("\\W").ReplaceAllString(url, "_")
}

func downloadToDataUri(url string) (string, string, error) {
	client := http.Client{Timeout: time.Second * 30}
	res, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	contentType := res.Header.Get("Content-Type")
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "data:%s;base64,", contentType)
	if _, err := io.Copy(base64.NewEncoder(base64.StdEncoding, &buf), res.Body); err != nil {
		return "", "", err
	}
	return buf.String(), contentType, nil
}
