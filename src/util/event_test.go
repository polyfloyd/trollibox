package util

import (
	"context"
	"testing"
	"time"
)

func TestEmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var em Emitter

	l := em.Listen(ctx)
	em.Emit("test")

	select {
	case msg := <-l:
		if msg != "test" {
			t.Errorf("Event malformed: %v", msg)
			return
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Event was not emitted")
	}
}

func TestBufferedEmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var em Emitter
	em.Release = time.Millisecond * 50

	const REPEAT = 3

	l := em.Listen(ctx)
	for i := 0; i < REPEAT; i++ {
		em.Emit("test")
	}
	time.Sleep(time.Millisecond * 100)
	em.Emit("end")

	var numReceived uint
outer:
	for {
		select {
		case event := <-l:
			if event == "test" {
				numReceived++
			} else if event == "end" {
				break outer
			}
		case <-time.After(time.Millisecond * 500):
			t.Errorf("Event was not emitted")
			return
		}
	}

	if 1 != numReceived {
		t.Errorf("Event was repeated too many times: %v", numReceived)
		return
	}
}
