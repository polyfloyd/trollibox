package util

import (
	"reflect"
	"testing"
	"time"
)

// TestEventEmission may be used in unit tests to test whether some action
// causes an event to be emitted.
func TestEventEmission(t *testing.T, ev Eventer, event interface{}, trigger func()) {
	t.Helper()
	l := ev.Events().Listen()
	defer ev.Events().Unlisten(l)
	trigger()
	for {
		select {
		case msg := <-l:
			t.Logf("%T %#v", msg, msg)
			if reflect.DeepEqual(msg, event) {
				return
			}
		case <-time.After(time.Second):
			t.Fatalf("Event %#v was not emitted", event)
		}
	}
}
