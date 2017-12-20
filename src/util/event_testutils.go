package util

import (
	"reflect"
	"testing"
	"time"
)

func TestEventEmission(t *testing.T, ev Eventer, event interface{}, trigger func()) {
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
