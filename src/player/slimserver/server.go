package slimserver

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"../../util"
)

type Server struct {
	connPool sync.Pool
	webUrl   string
}

func Connect(host string, port int, username, password *string, webUrl string) (*Server, error) {
	connect := func() (net.Conn, error) {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
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
				return nil, fmt.Errorf("Could not login")
			}
		}
		return conn, nil
	}

	serv := &Server{
		webUrl: webUrl,
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
	conn := serv.connPool.Get()
	if err, ok := conn.(error); ok {
		return nil, nil, err
	}
	return conn.(net.Conn), func() {
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
		return nil, fmt.Errorf("Unable to scan response")
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

// Retrieves a list of all players this server controls.
func (serv *Server) Players() ([]*Player, error) {
	res, err := serv.request("player", "count", "?")
	if err != nil {
		return nil, err
	}

	numPlayers, _ := strconv.ParseInt(res[2], 10, 32)
	if numPlayers == 0 {
		return []*Player{}, nil
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

		players = append(players, &Player{
			ID:      attrs["playerid"],
			Name:    attrs["name"],
			Model:   attrs["model"],
			Serv:    serv,
			Emitter: util.NewEmitter(),
		})
	}

	for _, pl := range players {
		if err := pl.reloadPlaylist(); err != nil {
			return nil, err
		}
		// Add a way to halt the eventLoop?
		go pl.eventLoop()
	}
	return players, nil
}

func queryEscape(str string) string {
	str = url.QueryEscape(str)
	str = strings.Replace(str, "+", "%20", -1)
	str = strings.Replace(str, "%26", "&", -1)
	return str
}

func encodeUri(uri string) string {
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
