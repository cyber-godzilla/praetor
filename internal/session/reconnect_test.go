package session

import (
	"testing"
	"time"
)

func TestReconnector_ExponentialBackoff(t *testing.T) {
	r := NewReconnector(1*time.Second, 30*time.Second, 2)

	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second, // capped
		30 * time.Second, // stays capped
	}

	for i, want := range expected {
		got := r.NextDelay()
		if got != want {
			t.Errorf("attempt %d: got %v, want %v", i, got, want)
		}
	}
}

func TestReconnector_AttemptCounter(t *testing.T) {
	r := NewReconnector(1*time.Second, 60*time.Second, 2)

	if r.Attempt() != 0 {
		t.Errorf("expected initial attempt 0, got %d", r.Attempt())
	}

	r.NextDelay()
	if r.Attempt() != 1 {
		t.Errorf("expected attempt 1 after first NextDelay, got %d", r.Attempt())
	}

	r.NextDelay()
	r.NextDelay()
	if r.Attempt() != 3 {
		t.Errorf("expected attempt 3, got %d", r.Attempt())
	}
}

func TestReconnector_Reset(t *testing.T) {
	r := NewReconnector(1*time.Second, 60*time.Second, 2)

	// Advance a few times
	r.NextDelay() // 1s, advance to 2s
	r.NextDelay() // 2s, advance to 4s
	r.NextDelay() // 4s, advance to 8s

	if r.Attempt() != 3 {
		t.Fatalf("expected attempt 3, got %d", r.Attempt())
	}

	r.Reset()

	if r.Attempt() != 0 {
		t.Errorf("expected attempt 0 after reset, got %d", r.Attempt())
	}

	// Should start from initial delay again
	got := r.NextDelay()
	if got != 1*time.Second {
		t.Errorf("expected 1s after reset, got %v", got)
	}
}

func TestReconnector_CapAtMax(t *testing.T) {
	r := NewReconnector(500*time.Millisecond, 2*time.Second, 3)

	// 500ms -> 1500ms -> 4500ms (capped to 2s)
	d1 := r.NextDelay()
	if d1 != 500*time.Millisecond {
		t.Errorf("expected 500ms, got %v", d1)
	}

	d2 := r.NextDelay()
	if d2 != 1500*time.Millisecond {
		t.Errorf("expected 1500ms, got %v", d2)
	}

	d3 := r.NextDelay()
	if d3 != 2*time.Second {
		t.Errorf("expected 2s (capped), got %v", d3)
	}

	d4 := r.NextDelay()
	if d4 != 2*time.Second {
		t.Errorf("expected 2s (still capped), got %v", d4)
	}
}

func TestReconnector_Multiplier3(t *testing.T) {
	r := NewReconnector(100*time.Millisecond, 10*time.Second, 3)

	expected := []time.Duration{
		100 * time.Millisecond,
		300 * time.Millisecond,
		900 * time.Millisecond,
		2700 * time.Millisecond,
		8100 * time.Millisecond,
		10 * time.Second, // capped
	}

	for i, want := range expected {
		got := r.NextDelay()
		if got != want {
			t.Errorf("attempt %d: got %v, want %v", i, got, want)
		}
	}
}
