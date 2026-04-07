package engine

import (
	"encoding/json"
	"sync"
	"time"
)

// MetricEntry is a single named metric with a display label and value.
type MetricEntry struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

// MetricSession tracks mode-declared metrics for a single session.
type MetricSession struct {
	Mode      string        `json:"mode"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time,omitempty"`
	Entries   []MetricEntry `json:"entries"`
}

// Duration returns the session duration.
func (ms *MetricSession) Duration() time.Duration {
	if ms.EndTime.IsZero() {
		return time.Since(ms.StartTime)
	}
	return ms.EndTime.Sub(ms.StartTime)
}

// JSON returns the session as a JSON byte slice.
func (ms *MetricSession) JSON() ([]byte, error) {
	return json.Marshal(ms)
}

// findEntry returns the index of an entry by key, or -1 if not found.
func (ms *MetricSession) findEntry(key string) int {
	for i, e := range ms.Entries {
		if e.Key == key {
			return i
		}
	}
	return -1
}

// Metrics tracks mode-declared metrics with session history.
type Metrics struct {
	mu      sync.Mutex
	current *MetricSession
	history []MetricSession
}

// NewMetrics creates a new Metrics tracker.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// StartSession begins a new metric session for the given mode.
// Ends the current session first if one exists.
func (m *Metrics) StartSession(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current != nil {
		m.current.EndTime = time.Now()
		m.history = append(m.history, *m.current)
	}

	m.current = &MetricSession{
		Mode:      mode,
		StartTime: time.Now(),
	}
}

// EndSession ends the current session and adds it to history.
func (m *Metrics) EndSession() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return
	}

	m.current.EndTime = time.Now()
	m.history = append(m.history, *m.current)
	m.current = nil
}

// Track declares a metric for the current session. If no session is
// active, one is started automatically. If the metric already exists,
// the label is updated. Initial value is 0.
func (m *Metrics) Track(key, label string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		m.current = &MetricSession{
			StartTime: time.Now(),
		}
	}

	idx := m.current.findEntry(key)
	if idx >= 0 {
		m.current.Entries[idx].Label = label
		return
	}

	m.current.Entries = append(m.current.Entries, MetricEntry{
		Key:   key,
		Label: label,
		Value: 0,
	})
}

// Inc increments a metric by 1.
func (m *Metrics) Inc(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return
	}
	idx := m.current.findEntry(key)
	if idx >= 0 {
		m.current.Entries[idx].Value++
	}
}

// Dec decrements a metric by 1.
func (m *Metrics) Dec(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return
	}
	idx := m.current.findEntry(key)
	if idx >= 0 {
		m.current.Entries[idx].Value--
	}
}

// Set sets a metric to a specific value.
func (m *Metrics) Set(key string, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return
	}
	idx := m.current.findEntry(key)
	if idx >= 0 {
		m.current.Entries[idx].Value = value
	}
}

// Get returns the value of a metric, or 0 if not found.
func (m *Metrics) Get(key string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return 0
	}
	idx := m.current.findEntry(key)
	if idx >= 0 {
		return m.current.Entries[idx].Value
	}
	return 0
}

// Current returns a copy of the current active session, or nil.
func (m *Metrics) Current() *MetricSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return nil
	}
	cp := *m.current
	cp.Entries = make([]MetricEntry, len(m.current.Entries))
	copy(cp.Entries, m.current.Entries)
	return &cp
}

// History returns a copy of all completed sessions.
func (m *Metrics) History() []MetricSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.history) == 0 {
		return nil
	}

	cp := make([]MetricSession, len(m.history))
	copy(cp, m.history)
	return cp
}
