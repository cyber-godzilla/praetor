package engine

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestQueueTelemetryAccountsForOutcomesAndDrops(t *testing.T) {
	q := NewCommandQueue(2, time.Millisecond, time.Millisecond, []string{"priority", "priority2", "priority3"})
	q.Enqueue("normal", 0)
	q.Enqueue("normal", 0) // duplicate
	q.Enqueue("other", 0)
	q.Enqueue("full", 0)      // full
	q.Enqueue("priority", 0)  // evicts other
	q.Enqueue("priority2", 0) // evicts normal
	q.Enqueue("priority3", 0) // full of priority commands
	cmd, _, ok := q.DequeueGen()
	if !ok || cmd.QueuedAt.IsZero() {
		t.Fatal("missing admission timestamp")
	}
	q.RecordDelivery(cmd, "sent", 2*time.Millisecond, 3*time.Millisecond, 4*time.Millisecond, 5*time.Millisecond)
	q.Clear()
	m := q.telemetry
	if m.admitted != 4 || m.dequeued != 1 || m.sent != 1 || m.cleared != 1 || m.peak != 2 || m.inFlight != 0 {
		t.Fatalf("incorrect queue accounting: %+v", m)
	}
	if m.duplicate != 1 || m.full != 1 || m.fullPriority != 1 || m.evicted != 2 {
		t.Fatalf("incorrect drop accounting: %+v", m)
	}
	if m.backlog != 2*time.Millisecond || m.pacingDelay != 3*time.Millisecond || m.intervalWait != 4*time.Millisecond || m.sendTime != 5*time.Millisecond {
		t.Fatalf("incorrect duration accounting: %+v", m)
	}
	for _, outcome := range []string{"cancelled", "failed"} {
		q.Enqueue(outcome, 0)
		cmd, _, _ := q.DequeueGen()
		q.RecordDelivery(cmd, outcome, 0, 0, 0, 0)
	}
	if q.telemetry.cancelled != 1 || q.telemetry.failed != 1 || q.telemetry.inFlight != 0 {
		t.Fatalf("incorrect terminal accounting: %+v", q.telemetry)
	}
}

func TestQueueTelemetryLogsOnlyChangedSnapshotsWithoutCommandText(t *testing.T) {
	var output bytes.Buffer
	old := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(old) })
	q := NewCommandQueue(10, time.Second, 500*time.Millisecond, nil)
	q.LogTelemetry()
	if output.Len() != 0 {
		t.Fatal("idle queue logged")
	}
	q.Enqueue("private command text", 0)
	q.LogTelemetry()
	first := output.String()
	if !strings.Contains(first, "[QUEUE:METRICS]") || !strings.Contains(first, "depth=1 capacity=10 peak=1") || !strings.Contains(first, "default_delay_ms=1000.000 min_interval_ms=500.000") {
		t.Fatalf("missing diagnostic fields: %s", first)
	}
	if strings.Contains(first, "private command text") {
		t.Fatal("telemetry leaked command text")
	}
	q.LogTelemetry()
	if output.String() != first {
		t.Fatal("unchanged snapshot logged again")
	}
}

func TestQueueTelemetryEnqueueToSendExcludesUnsentCommands(t *testing.T) {
	q := NewCommandQueue(10, 0, 0, nil)
	for _, age := range []time.Duration{20 * time.Millisecond, 40 * time.Millisecond} {
		q.Enqueue("test", 0)
		cmd, _, _ := q.DequeueGen()
		cmd.QueuedAt = time.Now().Add(-age)
		q.RecordDelivery(cmd, "sent", 0, 0, 0, 0)
	}
	m := q.telemetry
	if m.enqueueToSend < 60*time.Millisecond || m.maxEnqueueToSend < 40*time.Millisecond || m.lastEnqueueToSend < 40*time.Millisecond || m.sent != 2 {
		t.Fatalf("missing enqueue-to-send measurements: %+v", m)
	}
	for _, outcome := range []string{"cancelled", "failed"} {
		q.Enqueue(outcome, 0)
		cmd, _, _ := q.DequeueGen()
		cmd.QueuedAt = time.Now().Add(-time.Hour)
		q.RecordDelivery(cmd, outcome, 0, 0, 0, 0)
	}
	after := q.telemetry
	if after.enqueueToSend != m.enqueueToSend || after.maxEnqueueToSend != m.maxEnqueueToSend || after.lastEnqueueToSend != m.lastEnqueueToSend || after.sent != m.sent {
		t.Fatal("unsent commands contaminated enqueue-to-send latency")
	}
	var output bytes.Buffer
	old := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(old) })
	q.LogTelemetry()
	for _, key := range []string{"enqueue_to_send_ms=", "avg_enqueue_to_send_ms=", "max_enqueue_to_send_ms=", "last_enqueue_to_send_ms="} {
		if !strings.Contains(output.String(), key) {
			t.Fatalf("missing log field %s", key)
		}
	}
}
