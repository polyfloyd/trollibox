package util

import (
	"sync"
	"time"
)

type Emitter struct {
	// The release attribute determines how much time the event should be
	// buffered to prevent the emission of duplicate events.
	// A zero value will disable buffering.
	Release time.Duration

	listeners       map[<-chan string]chan string
	listenerClosers map[<-chan string]chan struct{}
	lock            sync.RWMutex

	release map[string]struct{}
}

func (emitter *Emitter) init() {
	emitter.lock.RLock()
	shouldInit := emitter.listeners == nil
	emitter.lock.RUnlock()
	if shouldInit {
		emitter.lock.Lock()
		if emitter.listeners == nil {
			emitter.listeners = map[<-chan string]chan string{}
			emitter.listenerClosers = map[<-chan string]chan struct{}{}
			emitter.release = map[string]struct{}{}
		}
		emitter.lock.Unlock()
	}
}

func (emitter *Emitter) broadcast(event string) {
	emitter.lock.RLock()
	defer emitter.lock.RUnlock()
	for _, listener := range emitter.listeners {
		go func(listener chan string) {
			select {
			case listener <- event:
			case <-emitter.listenerClosers[listener]:
			}
		}(listener)
	}
}

func (emitter *Emitter) Emit(event string) {
	emitter.init()

	emitter.lock.RLock()
	defer emitter.lock.RUnlock()

	if emitter.Release == 0 {
		go emitter.broadcast(event)
		return
	}

	// Check wether the event is already scheduled.
	if _, ok := emitter.release[event]; ok {
		return
	}

	go func() {
		emitter.lock.Lock()
		emitter.release[event] = struct{}{}
		emitter.lock.Unlock()

		time.Sleep(emitter.Release)
		emitter.broadcast(event)

		emitter.lock.Lock()
		delete(emitter.release, event)
		emitter.lock.Unlock()
	}()
}

func (emitter *Emitter) Listen() <-chan string {
	emitter.init()

	emitter.lock.Lock()
	defer emitter.lock.Unlock()

	ch := make(chan string, 1)
	emitter.listeners[ch] = ch
	emitter.listenerClosers[ch] = make(chan struct{})
	return ch
}

func (emitter *Emitter) Unlisten(ch <-chan string) {
	emitter.init()

	emitter.lock.Lock()
	defer emitter.lock.Unlock()

	// Signal any remaining broadcasts to abort writing to the channel.
	close(emitter.listenerClosers[ch])

	// Ok, now clean up everything.
	close(emitter.listeners[ch])
	delete(emitter.listenerClosers, ch)
	delete(emitter.listeners, ch)
}
