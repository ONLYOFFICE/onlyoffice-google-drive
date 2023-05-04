package events

import (
	"github.com/gookit/event"
)

type gooKitEmitter struct{}

func NewGoKitEmitter() Emitter {
	return &gooKitEmitter{}
}

func (g gooKitEmitter) On(name string, listener Listener) {
	event.On(name, event.ListenerFunc(func(e event.Event) error {
		return listener.Handle(e)
	}))
}

func (g gooKitEmitter) Fire(name string, payload map[string]any) {
	event.Fire(name, payload)
}
