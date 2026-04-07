package engine

import (
	"sync"
	"time"
)

// QueuedCommand represents a command waiting to be sent with its delay.
type QueuedCommand struct {
	Command string
	Delay   time.Duration
}

// CommandQueue is a thread-safe command queue with priority, dedup, and size limits.
type CommandQueue struct {
	mu           sync.Mutex
	items        []QueuedCommand
	maxSize      int
	defaultDelay time.Duration
	minInterval  time.Duration
	highPriority map[string]bool
	lastSendTime time.Time
}

// NewCommandQueue creates a new CommandQueue with the given configuration.
// highPriority commands are inserted at the front of the queue (after other
// high-priority items). Duplicate commands are dropped. Queue rejects new
// items when at maxSize.
func NewCommandQueue(maxSize int, defaultDelay, minInterval time.Duration, highPriority []string) *CommandQueue {
	hp := make(map[string]bool, len(highPriority))
	for _, cmd := range highPriority {
		hp[cmd] = true
	}
	return &CommandQueue{
		items:        make([]QueuedCommand, 0, maxSize),
		maxSize:      maxSize,
		defaultDelay: defaultDelay,
		minInterval:  minInterval,
		highPriority: hp,
	}
}

// SetHighPriority replaces the high-priority command list.
func (q *CommandQueue) SetHighPriority(cmds []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	hp := make(map[string]bool, len(cmds))
	for _, cmd := range cmds {
		hp[cmd] = true
	}
	q.highPriority = hp
}

// Enqueue adds a command to the queue. delayMs=0 uses the default delay.
// High-priority commands go to the front (after other high-priority items).
// Duplicate commands already in the queue are dropped.
// New items are dropped when queue is at maxSize.
func (q *CommandQueue) Enqueue(command string, delayMs int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Drop if at max size
	if len(q.items) >= q.maxSize {
		return
	}

	// Drop duplicates
	for _, item := range q.items {
		if item.Command == command {
			return
		}
	}

	delay := q.defaultDelay
	if delayMs > 0 {
		delay = time.Duration(delayMs) * time.Millisecond
	}

	qc := QueuedCommand{
		Command: command,
		Delay:   delay,
	}

	if q.highPriority[command] {
		// Insert after the last high-priority item
		insertIdx := 0
		for i, item := range q.items {
			if q.highPriority[item.Command] {
				insertIdx = i + 1
			}
		}
		// Insert at insertIdx
		q.items = append(q.items, QueuedCommand{})
		copy(q.items[insertIdx+1:], q.items[insertIdx:])
		q.items[insertIdx] = qc
	} else {
		q.items = append(q.items, qc)
	}
}

// Dequeue removes and returns the next command from the queue.
// Returns false if the queue is empty.
func (q *CommandQueue) Dequeue() (QueuedCommand, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return QueuedCommand{}, false
	}

	cmd := q.items[0]
	q.items = q.items[1:]
	return cmd, true
}

// Clear removes all commands from the queue.
func (q *CommandQueue) Clear() {
	q.mu.Lock()
	q.items = q.items[:0]
	q.mu.Unlock()
}

// Len returns the number of commands in the queue.
func (q *CommandQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// MinInterval returns the minimum interval between sends.
func (q *CommandQueue) MinInterval() time.Duration {
	return q.minInterval
}

// RecordSend records the current time as the last send time.
func (q *CommandQueue) RecordSend() {
	q.mu.Lock()
	q.lastSendTime = time.Now()
	q.mu.Unlock()
}

// TimeSinceLastSend returns time elapsed since the last send. If nothing has
// been sent yet, returns minInterval to allow an immediate first send.
func (q *CommandQueue) TimeSinceLastSend() time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.lastSendTime.IsZero() {
		return q.minInterval
	}
	return time.Since(q.lastSendTime)
}
