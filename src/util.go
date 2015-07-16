package main

import (
	"html/template"
	"io"
	"sync"
	assets "./assets-go"
)

const PAGE_BASE = "view/page.html"

var pageTemplates = map[string]*template.Template{}

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

type EventEmitter struct {
	listeners     map[uint64]chan string
	listenersEnum uint64
	listenersLock sync.Mutex
}

func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		listeners: map[uint64]chan string{},
	}
}

func (this *EventEmitter) Emit(event string) {
	this.listenersLock.Lock()
	for _, l := range this.listeners {
		l <- event
	}
	this.listenersLock.Unlock()
}

func (this *EventEmitter) Listen(listener chan string) uint64 {
	this.listenersLock.Lock()
	defer this.listenersLock.Unlock()

	this.listenersEnum++
	this.listeners[this.listenersEnum] = listener
	return this.listenersEnum
}

func (this *EventEmitter) Unlisten(handle uint64) {
	this.listenersLock.Lock()
	defer this.listenersLock.Unlock()

	delete(this.listeners, handle)
}
