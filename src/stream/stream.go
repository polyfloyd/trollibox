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

	"../player"
	"../util"
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
	m3u, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
	}
	defer m3u.Close()

	stream := &Stream{
		Filename: path.Base(filename),
	}
	extinfRe := regexp.MustCompile("^#EXTINF:\\d+,(.+)$")
	extartRe := regexp.MustCompile("^#EXTART:(data:[a-z]+/[a-z]+;base64,.+)$")

	scanner := bufio.NewScanner(m3u)
	for scanner.Scan() {
		line := scanner.Text()
		infMatch := extinfRe.FindStringSubmatch(line)
		if infMatch != nil {
			if stream.Title != "" {
				return nil, fmt.Errorf("Error loading stream from M3U: Duplicate title")
			}
			stream.Title = infMatch[1]
		}

		if artMatch := extartRe.FindStringSubmatch(line); artMatch != nil {
			if stream.ArtURI != "" {
				return nil, fmt.Errorf("Error loading stream from M3U: Duplicate art")
			}
			stream.ArtURI = artMatch[1]
		}
		if len(line) > 0 && line[0] != '#' {
			if stream.URL != "" {
				return nil, fmt.Errorf("Error loading stream from M3U: Duplicate URL")
			}
			stream.URL = line
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error loading stream from M3U: %v", err)
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
			if stream, err := LoadM3U(path.Join(db.directory, file.Name())); err == nil {
				streams = append(streams, *stream)
			} else {
				log.Printf("Unable to load stream from %q: %v", file.Name(), err)
			}
		}
	}
	return streams, nil
}

func (db *DB) RemoveStream(stream *Stream) error {
	if path.Ext(stream.Filename) != ".m3u" {
		return fmt.Errorf("Stream filenames must have the .m3u suffix")
	}
	defer db.Emit("update")
	return os.Remove(path.Join(db.directory, stream.Filename))
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

	fd, err := os.Create(path.Join(db.directory, stream.Filename))
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
