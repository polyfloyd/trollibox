package util

import (
	"sync"
)

type Emitter struct {
	listeners     map[chan string]bool
	listenersLock sync.Mutex
}

func NewEmitter() *Emitter {
	return &Emitter{listeners: map[chan string]bool{}}
}

func (emitter *Emitter) Emit(event string) {
	emitter.listenersLock.Lock()
	for l := range emitter.listeners {
		l <- event
	}
	emitter.listenersLock.Unlock()
}

func (emitter *Emitter) Listen() chan string {
	emitter.listenersLock.Lock()
	defer emitter.listenersLock.Unlock()

	ch := make(chan string, 16)
	emitter.listeners[ch] = true
	return ch
}

func (emitter *Emitter) Unlisten(ch chan string) {
	emitter.listenersLock.Lock()
	defer emitter.listenersLock.Unlock()

	close(ch)
	delete(emitter.listeners, ch)
}
