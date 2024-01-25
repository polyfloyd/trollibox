package eventsource

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
)

type EventSource struct {
	conn net.Conn
}

func Begin(w http.ResponseWriter, r *http.Request) (*EventSource, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Transfer-Encoding", "identity")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	conn, buf, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, fmt.Errorf("could not start event source: %v", err)
	}
	buf.Flush()

	go func() {
		<-r.Context().Done()
		conn.Close()
	}()

	return &EventSource{conn: conn}, nil
}

func (es *EventSource) Event(event, body string) {
	fmt.Fprintf(es.conn, "event: %s\n", event)
	if body != "" {
		fmt.Fprintf(es.conn, "data: %s\n\n", body)
	}
}

func (es *EventSource) EventJSON(event string, body interface{}) {
	b, err := json.Marshal(body)
	if err != nil {
		slog.Error("Could not marshal event", "event", event, "error", err)
		return
	}
	es.Event(event, string(b))
}
