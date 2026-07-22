package engine

import (
	"log"
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

	// gen counts Clear() calls. A drainer that pops a command (capturing the
	// generation via DequeueGen) and then sleeps on its delay compares the
	// generation on waking: if Clear() ran in between (mode switch), the command
	// belongs to a retired generation and is dropped rather than sent.
	gen uint64
	// notify is a coalescing wakeup for a single long-lived drainer. Enqueue
	// does a non-blocking send so the drainer, parked on Notify(), wakes and
	// drains promptly even on an idle connection (no incoming game text).
	notify chan struct{}

	// dropped counts commands the queue refused (full, duplicate, or evicted for
	// priority). Exposed via Dropped() so drops are observable instead of silent.
	dropped     uint64
	lastDropLog map[string]time.Time
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
		notify:       make(chan struct{}, 1),
		lastDropLog:  make(map[string]time.Time),
	}
}

// Dropped returns the total number of commands the queue has refused (full,
// duplicate, or evicted to make room for a high-priority command).
func (q *CommandQueue) Dropped() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dropped
}

// recordDrop counts a refused command and logs it at warn, rate-limited to once
// per second per reason so a runaway script can't flood the log. Caller holds mu.
func (q *CommandQueue) recordDrop(command, reason string) {
	q.dropped++
	now := time.Now()
	if last, ok := q.lastDropLog[reason]; !ok || now.Sub(last) >= time.Second {
		q.lastDropLog[reason] = now
		log.Printf("[QUEUE] dropped command %q (%s)", command, reason)
	}
}

// signal wakes a drainer parked on Notify(). Non-blocking: the buffered slot
// coalesces multiple enqueues into one pending wakeup.
func (q *CommandQueue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Notify returns a channel that receives a value whenever a command is admitted
// to the queue. A single drainer parks on this between sends.
func (q *CommandQueue) Notify() <-chan struct{} {
	return q.notify
}

// Generation returns the current clear-generation counter. See DequeueGen.
func (q *CommandQueue) Generation() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.gen
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

	// Drop duplicates already queued. Checked before capacity so a repeat never
	// evicts anything.
	for _, item := range q.items {
		if item.Command == command {
			q.recordDrop(command, "duplicate")
			return
		}
	}

	// Capacity handling. A full queue drops a normal command, but an emergency
	// high-priority command (stand/flee/...) instead evicts the newest normal
	// command to make room — the one command that matters most must not vanish.
	if len(q.items) >= q.maxSize {
		if q.highPriority[command] {
			evicted := false
			for i := len(q.items) - 1; i >= 0; i-- {
				if !q.highPriority[q.items[i].Command] {
					q.recordDrop(q.items[i].Command, "evicted-for-priority")
					q.items = append(q.items[:i], q.items[i+1:]...)
					evicted = true
					break
				}
			}
			if !evicted {
				// Every slot is already a high-priority command; drop loudly.
				q.recordDrop(command, "full-all-priority")
				return
			}
		} else {
			q.recordDrop(command, "full")
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

	q.signal()
}

// Dequeue removes and returns the next command from the queue.
// Returns false if the queue is empty. Used by tests as the queue-inspection
// primitive; production uses DequeueGen (which also carries the generation).
func (q *CommandQueue) Dequeue() (QueuedCommand, bool) {
	cmd, _, ok := q.DequeueGen()
	return cmd, ok
}

// DequeueGen is Dequeue plus the queue generation the command belonged to. A
// drainer captures this generation atomically with the pop, then re-checks it
// against Generation() after sleeping on the command's delay; a mismatch means
// Clear() (mode switch) intervened and the command must be dropped, not sent.
func (q *CommandQueue) DequeueGen() (QueuedCommand, uint64, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return QueuedCommand{}, q.gen, false
	}

	cmd := q.items[0]
	q.items = q.items[1:]
	return cmd, q.gen, true
}

// Clear removes all commands from the queue and advances the generation so a
// command already dequeued by a sleeping drainer is recognized as stale.
func (q *CommandQueue) Clear() {
	q.mu.Lock()
	q.items = q.items[:0]
	q.gen++
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
