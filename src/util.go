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

func (this *EventEmitter) Emit(event string) {
	this.listenersLock.Lock()
	for l := range this.listeners {
		l <- event
	}
	this.listenersLock.Unlock()
}

func (this *EventEmitter) Listen() chan string {
	this.listenersLock.Lock()
	defer this.listenersLock.Unlock()

	ch := make(chan string, 16)
	this.listeners[ch] = true
	return ch
}

func (this *EventEmitter) Unlisten(ch chan string) {
	this.listenersLock.Lock()
	defer this.listenersLock.Unlock()

	close(ch)
	delete(this.listeners, ch)
}
