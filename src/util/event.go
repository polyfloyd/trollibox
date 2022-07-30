package util

import (
	"context"
	"sync"
	"time"
)

const chanBufferSize = 128

// Eventer specifies the functionality required for a type to emit events.
type Eventer interface {
	// Returns a reference to the associated event emitter. Never nil.
	Events() *Emitter
}

// Emitter is an asynchronous single producer multiple consumer broadcasting.
type Emitter struct {
	// The release attribute determines how much time the event should be
	// buffered to prevent the emission of duplicate events.
	// A zero value will disable deduplication.
	Release time.Duration

	listeners map[chan<- interface{}]struct{}
	lock      sync.RWMutex

	release map[interface{}]struct{}
}

func (emitter *Emitter) init() {
	emitter.lock.RLock()
	shouldInit := emitter.listeners == nil
	emitter.lock.RUnlock()
	if shouldInit {
		emitter.lock.Lock()
		if emitter.listeners == nil {
			emitter.listeners = map[chan<- interface{}]struct{}{}
			emitter.release = map[interface{}]struct{}{}
		}
		emitter.lock.Unlock()
	}
}

func (emitter *Emitter) broadcast(event interface{}) {
	emitter.lock.RLock()
	defer emitter.lock.RUnlock()
	for listener := range emitter.listeners {
		select {
		case listener <- event:
		default:
		}
	}
}

// Emit emits an event to all current consumers.
//
// Listening channels are buffered, but whether the event is delivered
// dependending on the whether the receiving channel is being actively read by
// some goroutine.
func (emitter *Emitter) Emit(event interface{}) {
	emitter.init()

	emitter.lock.RLock()
	if emitter.Release == 0 {
		emitter.lock.RUnlock()
		emitter.broadcast(event)
		return
	}

	// Check whether the event is already scheduled.
	if _, ok := emitter.release[event]; ok {
		emitter.lock.RUnlock()
		return
	}
	emitter.lock.RUnlock()
	emitter.lock.Lock()
	emitter.release[event] = struct{}{}
	emitter.lock.Unlock()

	go func() {
		time.Sleep(emitter.Release)
		emitter.broadcast(event)

		emitter.lock.Lock()
		delete(emitter.release, event)
		emitter.lock.Unlock()
	}()
}

// Listen registers a new channel at this emitter.
//
// The returned channel remains open until the specified context is cancelled.
func (emitter *Emitter) Listen(ctx context.Context) <-chan interface{} {
	emitter.init()

	emitter.lock.Lock()
	defer emitter.lock.Unlock()

	ch := make(chan interface{}, chanBufferSize)
	emitter.listeners[ch] = struct{}{}

	go func() {
		<-ctx.Done()
		emitter.lock.Lock()
		defer emitter.lock.Unlock()
		close(ch)
		delete(emitter.listeners, ch)
	}()

	return ch
}
