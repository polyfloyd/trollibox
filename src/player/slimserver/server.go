package slimserver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/polyfloyd/trollibox/src/library"
	"github.com/polyfloyd/trollibox/src/util"
)

// Server handles connectivity to a Logitech SlimServer.
type Server struct {
	connPool sync.Pool
	webURL   string
}

// Connect connects to Logitech SlimServer with an optional username and password.
//
// webURL is used for album art and should be the location at which the regular
// webinterface of SlimServer can be reached.
func Connect(network, address string, username, password *string, webURL string) (*Server, error) {
	connect := func() (net.Conn, error) {
		conn, err := net.Dial(network, address)
		if err != nil {
			return nil, err
		}

		if username != nil && password != nil {
			conn.Write([]byte(fmt.Sprintf(
				"login %s %s\n",
				queryEscape(*username),
				queryEscape(*password),
			)))
			if scanner := bufio.NewScanner(conn); !scanner.Scan() {
				return nil, fmt.Errorf("could not login")
			}
		}
		return conn, nil
	}

	serv := &Server{
		webURL: webURL,
		connPool: sync.Pool{
			New: func() interface{} {
				conn, err := connect()
				if err != nil {
					return err
				}
				return conn
			},
		},
	}

	// Test connection.
	conn, err := connect()
	if err != nil {
		return nil, err
	}
	conn.Close()

	return serv, nil
}

func (serv *Server) conn() (net.Conn, func(), error) {
	maybeConn := serv.connPool.Get()
	if err, ok := maybeConn.(error); ok {
		return nil, nil, err
	}
	conn := maybeConn.(net.Conn)
	return conn, func() {
		serv.connPool.Put(conn)
	}, nil
}

func (serv *Server) requestRaw(p0 string, pn ...string) (net.Conn, func(), error) {
	conn, release, err := serv.conn()
	if err != nil {
		return nil, nil, err
	}

	// Write the request.
	conn.Write([]byte(queryEscape(p0)))
	for _, param := range pn {
		conn.Write([]byte(" " + queryEscape(param)))
	}
	if _, err := conn.Write([]byte("\n")); err != nil {
		conn.Close()
		return nil, nil, err
	}

	return conn, release, nil
}

func (serv *Server) request(p0 string, pn ...string) ([]string, error) {
	conn, release, err := serv.requestRaw(p0, pn...)
	if err != nil {
		return nil, err
	}
	defer release()

	// Read the LF delimited response.
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil, fmt.Errorf("unable to scan response")
	}
	response := scanner.Text()
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Format the response. In some cases, the command is replied followed by
	// two spaces.
	r := strings.Split(response, "  ")
	parts := strings.Split(r[len(r)-1], " ")
	decoded := make([]string, len(parts))
	for i, part := range parts {
		e, err := url.QueryUnescape(part)
		if err != nil {
			return nil, err
		}
		decoded[i] = e
	}
	return decoded, nil
}

func (serv *Server) requestAttrs(p0 string, pn ...string) (map[string]string, error) {
	res, err := serv.request(p0, pn...)
	if err != nil {
		return nil, err
	}

	attrs := map[string]string{}
	for _, str := range res {
		if s := strings.SplitN(str, ":", 2); len(s) == 2 {
			attrs[s[0]] = s[1]
		}
	}
	return attrs, nil
}

// Players retrieves a list of all players this server controls.
func (serv *Server) Players() ([]*Player, error) {
	res, err := serv.request("player", "count", "?")
	if err != nil {
		return nil, err
	}
	numPlayers, err := strconv.Atoi(res[2])
	if err != nil {
		return nil, err
	}

	players := make([]*Player, 0, numPlayers)
	for i := 0; i < int(numPlayers); i++ {
		attrs, err := serv.requestAttrs("players", strconv.Itoa(i), "1")
		if err != nil {
			return nil, err
		}
		if attrs["isplayer"] != "1" {
			continue
		}

		pl := &Player{
			ID:      attrs["playerid"],
			Name:    attrs["name"],
			Model:   attrs["model"],
			Serv:    serv,
			Emitter: util.Emitter{Release: time.Millisecond * 100},
		}
		pl.playlist.Playlist = slimPlaylist{player: pl}
		go pl.eventLoop() // Add a way to halt the eventLoop?
		players = append(players, pl)
	}
	return players, nil
}

func (serv *Server) decodeTracks(firstField string, numTracks int, p0 string, pn ...string) ([]library.Track, error) {
	if numTracks == 0 {
		return []library.Track{}, nil
	}

	reader, release, err := serv.requestRaw(p0, pn...)
	if err != nil {
		return nil, err
	}
	defer release()

	scanner := bufio.NewScanner(reader)
	// Set a custom scanner to split on spaces and newlines. atEOF is ignored
	// since the reader does not end.
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, ' '); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, data[0:i], io.EOF
		}
		return 0, nil, nil
	})

	for i := 0; i < 1+len(pn); i++ {
		scanner.Scan()
	}
	for scanner.Scan() {
		if tag, err := url.QueryUnescape(scanner.Text()); err == nil {
			if split := strings.SplitN(tag, ":", 2); split[0] == firstField {
				break
			}
		}
	}

	setAttr := func(tracks *[]library.Track, track **library.Track, field string) {
		tag, _ := url.QueryUnescape(scanner.Text())
		split := strings.SplitN(tag, ":", 2)
		if split[0] == firstField {
			if *track != nil {
				*tracks = append(*tracks, **track)
			}
			*track = &library.Track{}
		}
		if track != nil {
			setSlimAttr(serv, *track, split[0], split[1])
		}
	}

	tracks := make([]library.Track, 0, numTracks)
	var track *library.Track
	setAttr(&tracks, &track, scanner.Text())
	for scanner.Scan() {
		setAttr(&tracks, &track, scanner.Text())
	}
	if track != nil {
		tracks = append(tracks, *track)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return tracks, nil
}

func setSlimAttr(serv *Server, track *library.Track, key, value string) {
	switch key {
	case "url":
		uri, _ := url.QueryUnescape(value)
		track.URI = uri
	case "artist":
		fallthrough
	case "trackartist":
		track.Artist = value
	case "title":
		track.Title = value
	case "genre":
		track.Genre = value
	case "album":
		if a := value; a != "No Album" {
			track.Album = a
		}
	case "albumartist":
		track.AlbumArtist = value
	case "tracknum":
		track.AlbumTrack = value
	case "disc":
		track.AlbumDisc = value
	case "duration":
		d, _ := strconv.ParseFloat(value, 64)
		track.Duration = time.Duration(d) * time.Second
	case "coverid":
		track.HasArt = serv.webURL != "" && value != ""
	}
}

func queryEscape(str string) string {
	str = url.QueryEscape(str)
	replace := map[string]string{
		"+":   "%20",
		"%21": "!",
		"%24": "$",
		"%26": "&",
		"%28": "(",
		"%29": ")",
		"%2A": "*",
		"%2B": "+",
		"%2C": ",",
		"%3A": ":",
		"%3D": "=",
		"%40": "@",
		"%5B": "[",
		"%5C": "]",
		"%3F": "?",
	}
	for r, n := range replace {
		str = strings.Replace(str, r, n, -1)
	}
	return str
}

func encodeURI(uri string) string {
	if uri == "" {
		return ""
	}
	i := strings.Index(uri, "://")
	schema, path := uri[:i], uri[i+3:]
	if path[0] == '/' {
		path = path[1:]
	}

	split := strings.Split(path, "/")
	encodedParts := make([]string, len(split))
	for i, part := range split {
		encodedParts[i] = queryEscape(part)
	}

	var join string
	if schema == "file" {
		join = ":///"
	} else {
		join = "://"
	}
	return schema + join + strings.Join(encodedParts, "/")
}
