package util

import (
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

	listeners map[<-chan interface{}]chan interface{}
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
			emitter.listeners = map[<-chan interface{}]chan interface{}{}
			emitter.release = map[interface{}]struct{}{}
		}
		emitter.lock.Unlock()
	}
}

func (emitter *Emitter) broadcast(event interface{}) {
	emitter.lock.RLock()
	defer emitter.lock.RUnlock()
	for _, listener := range emitter.listeners {
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
// The returned channel should be freed with Unlisten.
func (emitter *Emitter) Listen() <-chan interface{} {
	emitter.init()

	emitter.lock.Lock()
	defer emitter.lock.Unlock()

	ch := make(chan interface{}, chanBufferSize)
	emitter.listeners[ch] = ch
	return ch
}

// Unlisten unregisters a channel previously obtained by Listen and closes it.
func (emitter *Emitter) Unlisten(ch <-chan interface{}) {
	emitter.init()

	emitter.lock.Lock()
	defer emitter.lock.Unlock()

	// Ok, now clean up everything.
	close(emitter.listeners[ch])
	delete(emitter.listeners, ch)
}
