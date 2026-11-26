package engine

import (
	"log"
	"sync/atomic"
	"time"
)

var nextQueueTelemetryID atomic.Uint64

// Temporary queue diagnostics for the responsiveness investigation. Counters
// are cumulative for a queue's lifetime; durations are accumulated milliseconds.
// Keep command contents out of these info-level application logs.
type queueTelemetry struct {
	enqueueToSend, maxEnqueueToSend, lastEnqueueToSend   time.Duration
	started                                              time.Time
	id                                                   uint64
	revision, loggedRevision                             uint64
	admitted, dequeued, sent, cancelled, failed, cleared uint64
	duplicate, full, fullPriority, evicted               uint64
	peak                                                 int
	backlog, pacingDelay, intervalWait, sendTime, total  time.Duration
	maxBacklog, maxTotal                                 time.Duration
	inFlight                                             int
}

// RecordDelivery records one dequeued command's terminal outcome. backlog is
// measured at dequeue; total includes backlog, pacing, and the network write.
func (q *CommandQueue) RecordDelivery(cmd QueuedCommand, outcome string, backlog, pacingDelay, intervalWait, sendTime time.Duration) {
	total := time.Since(cmd.QueuedAt)
	q.mu.Lock()
	defer q.mu.Unlock()
	m := &q.telemetry
	m.revision++
	if m.inFlight > 0 {
		m.inFlight--
	}
	switch outcome {
	case "sent":
		m.sent++
		m.enqueueToSend += total
		m.lastEnqueueToSend = total
		if total > m.maxEnqueueToSend {
			m.maxEnqueueToSend = total
		}
	case "cancelled":
		m.cancelled++
	case "failed":
		m.failed++
	}
	m.backlog += backlog
	m.pacingDelay += pacingDelay
	m.intervalWait += intervalWait
	m.sendTime += sendTime
	m.total += total
	if backlog > m.maxBacklog {
		m.maxBacklog = backlog
	}
	if total > m.maxTotal {
		m.maxTotal = total
	}
}

// LogTelemetry emits at most one snapshot per changed revision. Call every
// five seconds from the connection drainer, and once when it stops. Snapshot
// under the queue lock, then write outside it so logging doesn't block enqueue.
func (q *CommandQueue) LogTelemetry() {
	q.mu.Lock()
	if q.telemetry.revision == q.telemetry.loggedRevision {
		q.mu.Unlock()
		return
	}
	q.telemetry.loggedRevision = q.telemetry.revision
	m := q.telemetry
	depth, capacity := len(q.items), q.maxSize
	delay, interval := q.defaultDelay, q.minInterval
	q.mu.Unlock()
	var averageEnqueueToSend float64
	if m.sent > 0 {
		averageEnqueueToSend = milliseconds(m.enqueueToSend) / float64(m.sent)
	}
	log.Printf("[QUEUE:METRICS] queue=%d elapsed_ms=%.3f depth=%d capacity=%d peak=%d in_flight=%d admitted=%d dequeued=%d sent=%d cancelled=%d failed=%d cleared=%d duplicate=%d full=%d full_priority=%d evicted=%d backlog_ms=%.3f max_backlog_ms=%.3f pacing_ms=%.3f interval_wait_ms=%.3f send_ms=%.3f total_ms=%.3f max_total_ms=%.3f enqueue_to_send_ms=%.3f avg_enqueue_to_send_ms=%.3f max_enqueue_to_send_ms=%.3f last_enqueue_to_send_ms=%.3f default_delay_ms=%.3f min_interval_ms=%.3f", m.id, milliseconds(time.Since(m.started)), depth, capacity, m.peak, m.inFlight, m.admitted, m.dequeued, m.sent, m.cancelled, m.failed, m.cleared, m.duplicate, m.full, m.fullPriority, m.evicted, milliseconds(m.backlog), milliseconds(m.maxBacklog), milliseconds(m.pacingDelay), milliseconds(m.intervalWait), milliseconds(m.sendTime), milliseconds(m.total), milliseconds(m.maxTotal), milliseconds(m.enqueueToSend), averageEnqueueToSend, milliseconds(m.maxEnqueueToSend), milliseconds(m.lastEnqueueToSend), milliseconds(delay), milliseconds(interval))
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
