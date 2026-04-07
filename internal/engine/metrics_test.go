package engine

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMetrics_TrackAndInc(t *testing.T) {
	m := NewMetrics()

	m.Track("kills", "Kills")
	m.Track("crits", "Crits")
	m.Inc("kills")
	m.Inc("kills")
	m.Inc("kills")
	m.Inc("crits")

	if m.Get("kills") != 3 {
		t.Errorf("kills = %d, want 3", m.Get("kills"))
	}
	if m.Get("crits") != 1 {
		t.Errorf("crits = %d, want 1", m.Get("crits"))
	}
}

func TestMetrics_SetAndDec(t *testing.T) {
	m := NewMetrics()

	m.Track("score", "Score")
	m.Set("score", 100)
	if m.Get("score") != 100 {
		t.Errorf("score = %d, want 100", m.Get("score"))
	}

	m.Dec("score")
	if m.Get("score") != 99 {
		t.Errorf("score after dec = %d, want 99", m.Get("score"))
	}
}

func TestMetrics_TrackAutoStartsSession(t *testing.T) {
	m := NewMetrics()

	if m.Current() != nil {
		t.Error("should have no session before Track")
	}

	m.Track("test", "Test")

	if m.Current() == nil {
		t.Error("Track should auto-start a session")
	}
}

func TestMetrics_EndSessionAddsToHistory(t *testing.T) {
	m := NewMetrics()

	m.StartSession("macro")
	m.Track("kills", "Kills")
	m.Inc("kills")
	m.Inc("kills")
	m.EndSession()

	if m.Current() != nil {
		t.Error("Current() should be nil after EndSession()")
	}

	history := m.History()
	if len(history) != 1 {
		t.Fatalf("History() len = %d, want 1", len(history))
	}
	if history[0].Mode != "macro" {
		t.Errorf("Mode = %q, want 'macro'", history[0].Mode)
	}
	kills := 0
	for _, e := range history[0].Entries {
		if e.Key == "kills" {
			kills = e.Value
		}
	}
	if kills != 2 {
		t.Errorf("history kills = %d, want 2", kills)
	}
}

func TestMetrics_DurationPositive(t *testing.T) {
	m := NewMetrics()
	m.StartSession("test")
	time.Sleep(5 * time.Millisecond)

	cur := m.Current()
	d := cur.Duration()
	if d <= 0 {
		t.Errorf("Duration() = %v, want positive", d)
	}
}

func TestMetrics_JSONSerializable(t *testing.T) {
	m := NewMetrics()
	m.StartSession("macro")
	m.Track("kills", "Kills")
	m.Track("crits", "Critical Hits")
	m.Inc("kills")
	m.Set("crits", 5)

	cur := m.Current()
	data, err := cur.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	var parsed MetricSession
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Mode != "macro" {
		t.Errorf("Mode = %q, want 'macro'", parsed.Mode)
	}
	if len(parsed.Entries) != 2 {
		t.Fatalf("Entries len = %d, want 2", len(parsed.Entries))
	}
}

func TestMetrics_NoActiveSession_NoPanic(t *testing.T) {
	m := NewMetrics()

	// These should not panic.
	m.Inc("kills")
	m.Dec("kills")
	m.Set("kills", 5)
	m.EndSession()

	if m.Get("kills") != 0 {
		t.Error("Get on no session should return 0")
	}
}

func TestMetrics_StartSessionEndsExisting(t *testing.T) {
	m := NewMetrics()

	m.StartSession("first")
	m.Track("kills", "Kills")
	m.Inc("kills")

	m.StartSession("second")

	history := m.History()
	if len(history) != 1 {
		t.Fatalf("History() len = %d, want 1", len(history))
	}
	if history[0].Mode != "first" {
		t.Errorf("Mode = %q, want 'first'", history[0].Mode)
	}

	cur := m.Current()
	if cur == nil {
		t.Fatal("Current() should not be nil")
	}
	if cur.Mode != "second" {
		t.Errorf("Mode = %q, want 'second'", cur.Mode)
	}
}

func TestMetrics_HistoryReturnsCopy(t *testing.T) {
	m := NewMetrics()

	m.StartSession("test")
	m.Track("x", "X")
	m.Inc("x")
	m.EndSession()

	h1 := m.History()
	h2 := m.History()

	if len(h1) != 1 || len(h2) != 1 {
		t.Fatal("expected 1 entry in both")
	}

	h1[0].Mode = "modified"
	h2Again := m.History()
	if h2Again[0].Mode == "modified" {
		t.Error("History() should return a copy")
	}
}

func TestMetrics_MultipleSessionsHistory(t *testing.T) {
	m := NewMetrics()

	m.StartSession("macro")
	m.Track("kills", "Kills")
	m.Inc("kills")
	m.Inc("kills")
	m.EndSession()

	m.StartSession("chain")
	m.Track("kills", "Kills")
	m.Inc("kills")
	m.EndSession()

	history := m.History()
	if len(history) != 2 {
		t.Fatalf("len = %d, want 2", len(history))
	}
}

func TestMetrics_GetUntracked(t *testing.T) {
	m := NewMetrics()
	m.StartSession("test")

	// Getting an untracked metric returns 0.
	if m.Get("nonexistent") != 0 {
		t.Error("Get untracked should return 0")
	}
}
