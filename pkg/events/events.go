package events

type Event interface {
	Name() string
	Get(key string) any
	Add(key string, val any)
	Abort(bool)
	IsAborted() bool
}

type Listener interface {
	Handle(e Event) error
}

type Emitter interface {
	On(name string, listener Listener)
	Fire(name string, payload map[string]any)
}

func NewEmitter() Emitter {
	return NewGoKitEmitter()
}
