package gui

import (
	"sync"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// Emitter delivers named events to the frontend. The real implementation
// (in gui/main.go) wraps wailsruntime.EventsEmit; tests use a fake.
type Emitter interface {
	// Emit sends a named event with an optional JSON-serializable payload.
	Emit(event string, data any)
}

// EventChannel is the single frontend event name carrying a batch of
// WireEvents in delivery order.
const EventChannel = "praetor:events"

// captureEmitter is a test/inspection Emitter that records everything.
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

// toWire converts a single core event into a WireEvent for the frontend.
// It returns (event, true) when the event should be forwarded as-is, or
// (_, false) when the event is handled internally (e.g. SKOOT room/wall
// data that becomes a rendered minimap image instead of a raw payload).
func toWire(ev types.Event) (WireEvent, bool) {
	switch e := ev.(type) {
	case types.GameTextEvent:
		return WireEvent{Kind: KindText, Text: &TextPayload{
			Text:      e.Text,
			Segments:  toSegments(e.Styled),
			Raw:       e.Raw,
			IsEcho:    e.IsEcho,
			Timestamp: unixMillis(e.Timestamp),
		}}, true

	case types.SuppressedGameTextEvent:
		return WireEvent{Kind: KindSuppressed, Suppressed: &SuppressPayload{
			Channel:           int(e.Channel),
			SourceName:        e.SourceName,
			PlaceholderText:   e.PlaceholderText,
			PlaceholderStyled: toSegments(e.PlaceholderStyled),
			OriginalText:      e.OriginalText,
			OriginalStyled:    toSegments(e.OriginalStyled),
			Timestamp:         unixMillis(e.Timestamp),
		}}, true

	case types.StatusUpdateEvent:
		return WireEvent{Kind: KindStatus, Status: toStatusPayload(e)}, true

	case types.SKOOTUpdateEvent:
		// Room/wall data is consumed by the caller to render the minimap
		// image; here we forward only the status-bar / lighting / exits parts.
		bars := toBarsPayload(e)
		if bars == nil {
			return WireEvent{}, false
		}
		return WireEvent{Kind: KindBars, Bars: bars}, true

	case types.ConnectedEvent:
		return WireEvent{Kind: KindConn, Conn: &ConnPayload{State: "connected"}}, true

	case types.DisconnectedEvent:
		return WireEvent{Kind: KindConn, Conn: &ConnPayload{State: "disconnected", Reason: e.Reason}}, true

	case types.NotificationEvent:
		return WireEvent{Kind: KindNotify, Notify: &NotifyPayload{Title: e.Title, Message: e.Message}}, true

	case types.ErrorEvent:
		msg := ""
		if e.Err != nil {
			msg = e.Err.Error()
		}
		return WireEvent{Kind: KindError, Error: &ErrorPayload{Context: e.Context, Error: msg}}, true

	case types.CommandEvent:
		return WireEvent{Kind: KindCommand, Command: e.Command}, true

	case types.WikiOpenMenuEvent:
		return WireEvent{Kind: KindOpenMenu, OpenMenu: "wiki"}, true
	case types.MapsOpenMenuEvent:
		return WireEvent{Kind: KindOpenMenu, OpenMenu: "maps"}, true
	case types.CalcOpenMenuEvent:
		return WireEvent{Kind: KindOpenMenu, OpenMenu: "calc"}, true

	case types.ModeChangeEvent:
		// Mode changes are always followed by a StatusUpdateEvent (see
		// client.Run); no separate wire event needed.
		return WireEvent{}, false

	case types.MapURLEvent:
		// Not used by the GUI minimap (we render from SKOOT room/wall data).
		return WireEvent{}, false

	default:
		return WireEvent{}, false
	}
}

func toStatusPayload(e types.StatusUpdateEvent) *StatusPayload {
	p := &StatusPayload{Mode: e.Mode}
	if e.MetricsCurrent != nil {
		p.Current = toMetricSession(*e.MetricsCurrent)
	}
	for _, h := range e.MetricsHistory {
		p.History = append(p.History, *toMetricSession(h))
	}
	return p
}

func toMetricSession(m types.MetricSnapshot) *MetricSession {
	out := &MetricSession{
		Mode:       m.Mode,
		Start:      unixMillis(m.Start),
		End:        unixMillis(m.End),
		DurationMs: m.Duration().Milliseconds(),
	}
	for _, e := range m.Entries {
		out.Entries = append(out.Entries, MetricEntry{Label: e.Label, Value: e.Value})
	}
	return out
}

// toBarsPayload extracts the status-bar / lighting / exit fields of a SKOOT
// update. Returns nil when the event carries none of them (e.g. a pure
// room/wall map update), so the caller can skip emitting an empty bars event.
func toBarsPayload(e types.SKOOTUpdateEvent) *BarsPayload {
	p := &BarsPayload{}
	any := false
	if e.Health != nil {
		p.HasHealth, p.Health, any = true, *e.Health, true
	}
	if e.Fatigue != nil {
		p.HasFatigue, p.Fatigue, any = true, *e.Fatigue, true
	}
	if e.Encumbrance != nil {
		p.HasEncumbrance, p.Encumbrance, any = true, *e.Encumbrance, true
	}
	if e.Satiation != nil {
		p.HasSatiation, p.Satiation, any = true, *e.Satiation, true
	}
	if e.Lighting != nil {
		p.HasLighting, p.Lighting, p.LightingRaw, any = true, int(*e.Lighting), e.LightingRaw, true
	}
	if !any {
		return nil
	}
	return p
}
