// Package gui contains the Wails-facing application facade for praetor's
// desktop GUI. It wraps the UI-agnostic client.Client, translates the
// internal event stream into JSON-friendly payloads for the web frontend,
// and exposes bound methods the frontend calls.
//
// This package deliberately does NOT import the Wails runtime. All emission
// happens through the Emitter interface, so the facade logic is fully unit
// testable without a webview. The thin Wails wiring lives in the nested
// module under gui/ (gui/main.go).
package gui

import (
	"time"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// Segment is the JSON-serializable form of types.StyledSegment.
type Segment struct {
	Text      string `json:"text"`
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
	Color     string `json:"color,omitempty"`
	IsHR      bool   `json:"isHR,omitempty"`
}

func toSegments(in []types.StyledSegment) []Segment {
	if len(in) == 0 {
		return nil
	}
	out := make([]Segment, len(in))
	for i, s := range in {
		out[i] = Segment{
			Text:      s.Text,
			Bold:      s.Bold,
			Italic:    s.Italic,
			Underline: s.Underline,
			Color:     s.Color,
			IsHR:      s.IsHR,
		}
	}
	return out
}

// Event kinds emitted to the frontend on the "praetor:events" channel.
const (
	KindText       = "text"       // game text line
	KindSuppressed = "suppressed" // ignorelist-suppressed line (placeholder + original)
	KindStatus     = "status"     // mode + display state + metrics
	KindBars       = "bars"       // status bar values (health/fatigue/etc) + lighting
	KindConn       = "conn"       // connection state change
	KindNotify     = "notify"     // desktop notification mirror
	KindError      = "error"      // error event
	KindCommand    = "command"    // a command was sent to the server (from queue)
	KindOpenMenu   = "openMenu"   // client asks the UI to open a menu (wiki/maps/calc)
	KindMinimap    = "minimap"    // minimap PNG (base64 data URI) — synthesized, not a core event
	KindCompass    = "compass"    // compass PNG (base64 data URI) — synthesized
	KindDebug      = "debug"      // raw SKOOT payload for the debug panel (debug mode only)
)

// WireEvent is a tagged union sent to the frontend. Exactly one of the
// pointer fields is set according to Kind. Events are delivered in order in
// batches to preserve text ordering while limiting IPC chatter.
type WireEvent struct {
	Kind       string           `json:"kind"`
	Text       *TextPayload     `json:"text,omitempty"`
	Suppressed *SuppressPayload `json:"suppressed,omitempty"`
	Status     *StatusPayload   `json:"status,omitempty"`
	Bars       *BarsPayload     `json:"bars,omitempty"`
	Conn       *ConnPayload     `json:"conn,omitempty"`
	Notify     *NotifyPayload   `json:"notify,omitempty"`
	Error      *ErrorPayload    `json:"error,omitempty"`
	Command    string           `json:"command,omitempty"`
	OpenMenu   string           `json:"openMenu,omitempty"`
	Image      *ImagePayload    `json:"image,omitempty"`
	Debug      *DebugPayload    `json:"debug,omitempty"`
}

// DebugPayload carries a raw SKOOT channel + payload for the debug panel.
type DebugPayload struct {
	Channel int    `json:"channel"`
	Payload string `json:"payload"`
}

// TextPayload carries a single game text line.
type TextPayload struct {
	Text      string    `json:"text"`
	Segments  []Segment `json:"segments"`
	Raw       string    `json:"raw,omitempty"`
	IsEcho    bool      `json:"isEcho,omitempty"`
	Timestamp int64     `json:"timestamp"` // unix millis
}

// SuppressPayload carries an ignorelist-suppressed line. The frontend renders
// the placeholder by default and can reveal the original on demand.
type SuppressPayload struct {
	Channel           int       `json:"channel"`
	SourceName        string    `json:"sourceName"`
	PlaceholderText   string    `json:"placeholderText"`
	PlaceholderStyled []Segment `json:"placeholderStyled"`
	OriginalText      string    `json:"originalText"`
	OriginalStyled    []Segment `json:"originalStyled"`
	Timestamp         int64     `json:"timestamp"`
}

// StateItem mirrors types.StateDisplayItem.
type StateItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// MetricEntry mirrors types.MetricSnapshotEntry.
type MetricEntry struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// MetricSession is the JSON form of types.MetricSnapshot.
type MetricSession struct {
	Mode       string        `json:"mode"`
	Start      int64         `json:"start"`      // unix millis
	End        int64         `json:"end"`        // unix millis, 0 if active
	DurationMs int64         `json:"durationMs"` // convenience, computed server-side
	Entries    []MetricEntry `json:"entries"`
}

// StatusPayload carries mode, per-mode display state, and metrics.
type StatusPayload struct {
	Mode         string          `json:"mode"`
	DisplayState []StateItem     `json:"displayState"`
	Current      *MetricSession  `json:"current,omitempty"`
	History      []MetricSession `json:"history,omitempty"`
}

// BarsPayload carries the SKOOT status bar values and lighting. Any nil field
// on the source event is omitted (pointer -> omitempty is not enough for ints,
// so we send a Has* flag alongside).
type BarsPayload struct {
	HasHealth      bool `json:"hasHealth"`
	Health         int  `json:"health"`
	HasFatigue     bool `json:"hasFatigue"`
	Fatigue        int  `json:"fatigue"`
	HasEncumbrance bool `json:"hasEncumbrance"`
	Encumbrance    int  `json:"encumbrance"`
	HasSatiation   bool `json:"hasSatiation"`
	Satiation      int  `json:"satiation"`
	HasLighting    bool `json:"hasLighting"`
	Lighting       int  `json:"lighting"`    // LightingLevel enum value
	LightingRaw    int  `json:"lightingRaw"` // raw SKOOT ch9 value
}

// ConnPayload describes a connection state transition.
type ConnPayload struct {
	State     string `json:"state"` // "connected", "disconnected", "reconnecting"
	Reason    string `json:"reason,omitempty"`
	Attempt   int    `json:"attempt,omitempty"`
	NextDelay int64  `json:"nextDelayMs,omitempty"`
}

// NotifyPayload mirrors a desktop notification.
type NotifyPayload struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

// ErrorPayload mirrors types.ErrorEvent.
type ErrorPayload struct {
	Context string `json:"context"`
	Error   string `json:"error"`
}

// ImagePayload carries a rendered raster (minimap/compass) as a base64
// PNG data URI, sized so the frontend can lay it out crisply.
type ImagePayload struct {
	DataURI string `json:"dataURI"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

func unixMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
