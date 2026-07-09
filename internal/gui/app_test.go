package gui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestToWire_GameText(t *testing.T) {
	ev := types.GameTextEvent{
		Text:      "You see a rat.",
		Styled:    []types.StyledSegment{{Text: "You see a rat.", Bold: true, Color: "#ff0000"}},
		Timestamp: time.UnixMilli(1234),
		IsEcho:    false,
	}
	w, ok := toWire(ev)
	if !ok {
		t.Fatal("expected game text to convert")
	}
	if w.Kind != KindText {
		t.Fatalf("kind = %q, want %q", w.Kind, KindText)
	}
	if w.Text == nil || w.Text.Text != "You see a rat." {
		t.Fatalf("text payload wrong: %+v", w.Text)
	}
	if len(w.Text.Segments) != 1 || !w.Text.Segments[0].Bold || w.Text.Segments[0].Color != "#ff0000" {
		t.Fatalf("segment not carried through: %+v", w.Text.Segments)
	}
	if w.Text.Timestamp != 1234 {
		t.Fatalf("timestamp = %d, want 1234", w.Text.Timestamp)
	}
}

func TestToWire_ConnStates(t *testing.T) {
	cases := []struct {
		ev    types.Event
		state string
	}{
		{types.ConnectedEvent{}, "connected"},
		{types.DisconnectedEvent{Reason: "closed"}, "disconnected"},
	}
	for _, c := range cases {
		w, ok := toWire(c.ev)
		if !ok || w.Kind != KindConn || w.Conn == nil {
			t.Fatalf("conn event %T did not convert: ok=%v", c.ev, ok)
		}
		if w.Conn.State != c.state {
			t.Errorf("%T: state = %q, want %q", c.ev, w.Conn.State, c.state)
		}
	}
}

func TestToBarsPayload(t *testing.T) {
	h, f := 40, 12
	e := types.SKOOTUpdateEvent{Health: &h, Fatigue: &f}
	p := toBarsPayload(e)
	if p == nil {
		t.Fatal("expected bars payload")
	}
	if !p.HasHealth || p.Health != 40 || !p.HasFatigue || p.Fatigue != 12 {
		t.Fatalf("bars wrong: %+v", p)
	}
	if p.HasEncumbrance || p.HasSatiation || p.HasLighting {
		t.Errorf("unset fields should not be flagged: %+v", p)
	}

	// A pure room/wall update carries no bars.
	if toBarsPayload(types.SKOOTUpdateEvent{Rooms: []types.MinimapRoom{{X: 1}}}) != nil {
		t.Error("room-only update should yield nil bars")
	}
}

func TestToWire_SuppressedCarriesBoth(t *testing.T) {
	ev := types.SuppressedGameTextEvent{
		Channel:         types.IgnoreChannelOOC,
		SourceName:      "someone",
		PlaceholderText: "[suppressed: someone OOC]",
		OriginalText:    "(OOC) hello",
		OriginalStyled:  []types.StyledSegment{{Text: "(OOC) hello"}},
	}
	w, ok := toWire(ev)
	if !ok || w.Kind != KindSuppressed || w.Suppressed == nil {
		t.Fatalf("suppressed did not convert: ok=%v kind=%q", ok, w.Kind)
	}
	if w.Suppressed.OriginalText != "(OOC) hello" || w.Suppressed.PlaceholderText == "" {
		t.Fatalf("suppressed payload wrong: %+v", w.Suppressed)
	}
}

func TestWithColorWords(t *testing.T) {
	// A game text line with a color word gets recolored.
	ev := types.GameTextEvent{
		Text:   "Some shimmering deep red leather boots",
		Styled: []types.StyledSegment{{Text: "Some shimmering deep red leather boots"}},
	}
	out, ok := withColorWords(ev).(types.GameTextEvent)
	if !ok {
		t.Fatal("expected GameTextEvent back")
	}
	colored := false
	for _, s := range out.Styled {
		if s.Color != "" {
			colored = true
		}
	}
	if !colored {
		t.Error("expected color words to add a colored segment")
	}

	// Non-text events pass through unchanged.
	if _, ok := withColorWords(types.ConnectedEvent{}).(types.ConnectedEvent); !ok {
		t.Error("non-text event should pass through unchanged")
	}
}

func TestEncodeImage_Nil(t *testing.T) {
	if encodeImage(nil) != nil {
		t.Error("nil image should encode to nil payload")
	}
}

func TestRenderer_Reset(t *testing.T) {
	r := newRenderer()
	r.haveExits = true
	r.exits = types.Exits{}
	r.mini.Update([]types.MinimapRoom{{X: 1, Y: 1, Size: 5}}, nil)

	r.reset()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.haveExits {
		t.Error("haveExits should be false after reset")
	}
}

func TestProcessBatch_DisconnectResetsGuiState(t *testing.T) {
	deps := &Deps{Config: config.Defaults()}
	em := &captureEmitter{}
	a := NewGuiApp(deps, em)

	// Seed the state a disconnect must clear.
	a.kudosPromptShown = true
	a.render.haveExits = true

	a.processBatch([]types.Event{types.DisconnectedEvent{Reason: "connection closed"}})

	if a.kudosPromptShown {
		t.Error("kudosPromptShown should be reset on disconnect")
	}
	a.render.mu.Lock()
	he := a.render.haveExits
	a.render.mu.Unlock()
	if he {
		t.Error("renderer haveExits should be cleared on disconnect")
	}

	// The disconnected event must still reach the frontend as a Conn wire event.
	found := false
	for _, emitted := range em.snapshot() {
		batch, ok := emitted.data.([]WireEvent)
		if !ok {
			continue
		}
		for _, w := range batch {
			if w.Kind == KindConn && w.Conn != nil && w.Conn.State == "disconnected" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a Conn wire event with State == \"disconnected\" to be emitted")
	}
}

func TestRenderer_CompassProducesPNG(t *testing.T) {
	r := newRenderer()
	img := r.updateExits(types.Exits{North: true, East: true})
	if img == nil {
		t.Fatal("expected compass image")
	}
	if img.Width <= 0 || img.Height <= 0 {
		t.Fatalf("bad dims: %dx%d", img.Width, img.Height)
	}
	if len(img.DataURI) < len("data:image/png;base64,")+10 {
		t.Fatalf("data URI too short: %q", img.DataURI)
	}
	if img.DataURI[:22] != "data:image/png;base64," {
		t.Fatalf("wrong data URI prefix: %q", img.DataURI[:22])
	}
}

func TestSetActionSets(t *testing.T) {
	dir := t.TempDir()
	deps := &Deps{
		Config:     config.Defaults(),
		ConfigPath: filepath.Join(dir, "config.yaml"),
	}
	a := NewGuiApp(deps, &captureEmitter{})

	sets := []config.ActionSet{
		{Name: "Combat", Buttons: []config.ActionButton{{Label: "Attack", Command: "attack"}}},
	}
	if err := a.SetActionSets(sets); err != nil {
		t.Fatalf("SetActionSets: %v", err)
	}

	// In-memory config updated.
	if len(deps.Config.UI.ActionSets) != 1 || deps.Config.UI.ActionSets[0].Name != "Combat" {
		t.Fatalf("in-memory config not updated: %+v", deps.Config.UI.ActionSets)
	}
	// Persisted to disk.
	got, err := config.Load(deps.ConfigPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got.UI.ActionSets) != 1 || got.UI.ActionSets[0].Buttons[0].Command != "attack" {
		t.Fatalf("persisted config wrong: %+v", got.UI.ActionSets)
	}
}
