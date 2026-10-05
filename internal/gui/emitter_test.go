package gui

import "sync"

// captureEmitter records frontend emissions for facade tests.
type captureEmitter struct {
	mu     sync.Mutex
	events []capturedEmit
}

type capturedEmit struct {
	name string
	data any
}

func (c *captureEmitter) Emit(event string, data any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, capturedEmit{name: event, data: data})
}

func (c *captureEmitter) snapshot() []capturedEmit {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]capturedEmit, len(c.events))
	copy(out, c.events)
	return out
}
