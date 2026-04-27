package types

import "time"

type StyledSegment struct {
	Text      string
	Bold      bool
	Italic    bool
	Underline bool
	Color     string
	IsHR      bool
}

type LightingLevel int

const (
	LightBlindinglyBright LightingLevel = iota
	LightVeryBright
	LightBright
	LightFairlyLit
	LightSomewhatDark
	LightVeryDark
	LightExtremelyDark
	LightPitchBlack
)

type Exits struct {
	North     bool
	Northeast bool
	East      bool
	Southeast bool
	South     bool
	Southwest bool
	West      bool
	Northwest bool
	Up        bool
	Down      bool
}

type Event interface {
	eventMarker()
}

type GameTextEvent struct {
	Text      string
	Styled    []StyledSegment
	Timestamp time.Time
	Raw       string
	IsEcho    bool // true for command-echo events (user-typed or script-sent)
}

func (GameTextEvent) eventMarker() {}

// MinimapRoom represents a room on the minimap.
type MinimapRoom struct {
	X, Y       int
	Size       int
	Color      string // "#ff0000" = player, "#ffffff" = other
	Brightness float64
}

// MinimapWall represents a wall/door segment on the minimap.
type MinimapWall struct {
	X, Y     int
	Type     string // "hor", "ver", "ne", "nw"
	Passable bool   // true = open passage, false = solid wall
}

type SKOOTUpdateEvent struct {
	Channel     int    // SKOOT channel number
	RawPayload  string // raw SKOOT payload string
	HelpURL     string // from SKOOT channel 5 — URL to open in browser
	Exits       *Exits
	Lighting    *LightingLevel
	LightingRaw int // raw SKOOT channel 9 value
	Health      *int
	Fatigue     *int
	Encumbrance *int
	Satiation   *int
	Rooms       []MinimapRoom // from SKOOT channel 6
	Walls       []MinimapWall // from SKOOT channel 10
}

func (SKOOTUpdateEvent) eventMarker() {}

type MapURLEvent struct {
	URL string
}

func (MapURLEvent) eventMarker() {}

type ModeChangeEvent struct {
	NewMode  string
	PrevMode string
}

func (ModeChangeEvent) eventMarker() {}

type StateDisplayItem struct {
	Label string
	Value string
}

// MetricSnapshot is a lightweight copy of a metric session for the UI.
type MetricSnapshot struct {
	Mode    string
	Start   time.Time
	End     time.Time // zero if still active
	Entries []MetricSnapshotEntry
}

// MetricSnapshotEntry is a single metric value.
type MetricSnapshotEntry struct {
	Label string
	Value int
}

// Duration returns the session duration.
func (ms *MetricSnapshot) Duration() time.Duration {
	if ms.End.IsZero() {
		return time.Since(ms.Start)
	}
	return ms.End.Sub(ms.Start)
}

type StatusUpdateEvent struct {
	Mode           string
	DisplayState   []StateDisplayItem
	MetricsCurrent *MetricSnapshot
	MetricsHistory []MetricSnapshot
}

func (StatusUpdateEvent) eventMarker() {}

type ConnectedEvent struct{}

func (ConnectedEvent) eventMarker() {}

type DisconnectedEvent struct {
	Reason string
}

func (DisconnectedEvent) eventMarker() {}

type ReconnectingEvent struct {
	Attempt   int
	NextDelay time.Duration
}

func (ReconnectingEvent) eventMarker() {}

type NotificationEvent struct {
	Title   string
	Message string
}

func (NotificationEvent) eventMarker() {}

type ErrorEvent struct {
	Context string
	Err     error
}

func (ErrorEvent) eventMarker() {}

type CommandEvent struct {
	Command string
}

func (CommandEvent) eventMarker() {}

// WikiOpenMenuEvent is sent by the /wiki command (bare, no key) to ask
// the TUI to open the wiki bookmark browser.
type WikiOpenMenuEvent struct{}

func (WikiOpenMenuEvent) eventMarker() {}

// MapsOpenMenuEvent triggers opening the maps bookmark menu in the TUI.
type MapsOpenMenuEvent struct{}

func (MapsOpenMenuEvent) eventMarker() {}
