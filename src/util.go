package main

import (
	"html/template"
	"io"
	"sync"
	"time"

	assets "./assets-go"
)

const PAGE_BASE = "view/page.html"

var (
	pageTemplates  = map[string]*template.Template{}
	httpCacheSince = time.Now()
)

func RenderPage(name string, wr io.Writer, data interface{}) error {
	return getPageTemplate(name).ExecuteTemplate(wr, "page", data)
}

func getPageTemplate(name string) *template.Template {
	if page, ok := pageTemplates[name]; !ok || BUILD == "debug" {
		base := template.Must(template.New(PAGE_BASE).Parse(string(assets.MustAsset(PAGE_BASE))))
		page = template.Must(base.New(name).Parse(string(assets.MustAsset(name))))
		pageTemplates[name] = page
		return page
	} else {
		return page
	}
}

func HttpCacheTime() time.Time {
	if BUILD == "debug" {
		return time.Now()
	} else {
		return httpCacheSince
	}
}

type EventEmitter struct {
	listeners     map[chan string]bool
	listenersLock sync.Mutex
}

func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		listeners: map[chan string]bool{},
	}
}

func (emitter *EventEmitter) Emit(event string) {
	emitter.listenersLock.Lock()
	for l := range emitter.listeners {
		l <- event
	}
	emitter.listenersLock.Unlock()
}

func (emitter *EventEmitter) Listen() chan string {
	emitter.listenersLock.Lock()
	defer emitter.listenersLock.Unlock()

	ch := make(chan string, 16)
	emitter.listeners[ch] = true
	return ch
}

func (emitter *EventEmitter) Unlisten(ch chan string) {
	emitter.listenersLock.Lock()
	defer emitter.listenersLock.Unlock()

	close(ch)
	delete(emitter.listeners, ch)
}
