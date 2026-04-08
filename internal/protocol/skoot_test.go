package protocol

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestParseSkoot_Valid(t *testing.T) {
	seq, payload, err := ParseSkoot("SKOOT 7 n,show,ne,none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seq != 7 {
		t.Errorf("expected seq 7, got %d", seq)
	}
	if payload != "n,show,ne,none" {
		t.Errorf("expected payload 'n,show,ne,none', got %q", payload)
	}
}

func TestParseSkoot_SeqZero(t *testing.T) {
	seq, payload, err := ParseSkoot("SKOOT 0 some payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seq != 0 {
		t.Errorf("expected seq 0, got %d", seq)
	}
	if payload != "some payload" {
		t.Errorf("expected 'some payload', got %q", payload)
	}
}

func TestParseSkoot_NoPrefix(t *testing.T) {
	_, _, err := ParseSkoot("NOT_A_SKOOT 1 data")
	if err == nil {
		t.Error("expected error for missing SKOOT prefix")
	}
}

func TestParseSkoot_NonNumericSeq(t *testing.T) {
	_, _, err := ParseSkoot("SKOOT abc payload")
	if err == nil {
		t.Error("expected error for non-numeric sequence")
	}
}

func TestParseSkoot_MissingPayload(t *testing.T) {
	_, _, err := ParseSkoot("SKOOT 1")
	if err == nil {
		t.Error("expected error for missing payload")
	}
}

// --- Exits (channel 7) ---

func TestInterpretSkoot_ExitsReal(t *testing.T) {
	// Real data from game log
	ev := InterpretSkoot(7, "n,show,ne,none,e,show,se,none,s,show,sw,none,w,show,nw,none,u,none,d,show")
	if ev == nil || ev.Exits == nil {
		t.Fatal("expected exits event")
	}
	if !ev.Exits.North {
		t.Error("North should be show")
	}
	if ev.Exits.Northeast {
		t.Error("Northeast should be none")
	}
	if !ev.Exits.East {
		t.Error("East should be show")
	}
	if !ev.Exits.South {
		t.Error("South should be show")
	}
	if !ev.Exits.West {
		t.Error("West should be show")
	}
	if !ev.Exits.Down {
		t.Error("Down should be show")
	}
	if ev.Exits.Up {
		t.Error("Up should be none")
	}
}

func TestInterpretSkoot_ExitsAllNone(t *testing.T) {
	ev := InterpretSkoot(7, "n,none,ne,none,e,none,se,none,s,show,sw,none,w,none,nw,none,u,none,d,none")
	if ev == nil || ev.Exits == nil {
		t.Fatal("expected exits event")
	}
	if ev.Exits.North || ev.Exits.East || ev.Exits.West {
		t.Error("only South should be true")
	}
	if !ev.Exits.South {
		t.Error("South should be show")
	}
}

// --- Status bars (channel 8) ---

func TestInterpretSkoot_Health(t *testing.T) {
	ev := InterpretSkoot(8, "Health,80")
	if ev == nil || ev.Health == nil {
		t.Fatal("expected health event")
	}
	if *ev.Health != 80 {
		t.Errorf("health = %d, want 80", *ev.Health)
	}
	if ev.Fatigue != nil || ev.Encumbrance != nil || ev.Satiation != nil {
		t.Error("other fields should be nil")
	}
}

func TestInterpretSkoot_Fatigue(t *testing.T) {
	ev := InterpretSkoot(8, "Fatigue,28")
	if ev == nil || ev.Fatigue == nil {
		t.Fatal("expected fatigue event")
	}
	if *ev.Fatigue != 28 {
		t.Errorf("fatigue = %d, want 28", *ev.Fatigue)
	}
}

func TestInterpretSkoot_Encumbrance(t *testing.T) {
	ev := InterpretSkoot(8, "Encumbrance,62")
	if ev == nil || ev.Encumbrance == nil {
		t.Fatal("expected encumbrance event")
	}
	if *ev.Encumbrance != 62 {
		t.Errorf("encumbrance = %d, want 62", *ev.Encumbrance)
	}
}

func TestInterpretSkoot_Satiation(t *testing.T) {
	ev := InterpretSkoot(8, "Satiation,37")
	if ev == nil || ev.Satiation == nil {
		t.Fatal("expected satiation event")
	}
	if *ev.Satiation != 37 {
		t.Errorf("satiation = %d, want 37", *ev.Satiation)
	}
}

func TestInterpretSkoot_StatusInvalidValue(t *testing.T) {
	ev := InterpretSkoot(8, "Health,notanumber")
	if ev != nil {
		t.Error("expected nil for non-numeric status value")
	}
}

func TestInterpretSkoot_StatusUnknownName(t *testing.T) {
	ev := InterpretSkoot(8, "Mana,50")
	if ev != nil {
		t.Error("expected nil for unknown status name")
	}
}

// --- Lighting (channel 9) ---

func TestInterpretSkoot_LightingBlindinglyBright(t *testing.T) {
	ev := InterpretSkoot(9, "150")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightBlindinglyBright {
		t.Errorf("expected LightBlindinglyBright for 150, got %d", *ev.Lighting)
	}
}

func TestInterpretSkoot_LightingVeryBright(t *testing.T) {
	ev := InterpretSkoot(9, "90")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightVeryBright {
		t.Errorf("expected LightVeryBright for 90, got %d", *ev.Lighting)
	}
}

func TestInterpretSkoot_LightingBright(t *testing.T) {
	ev := InterpretSkoot(9, "60")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightBright {
		t.Errorf("expected LightBright for 60, got %d", *ev.Lighting)
	}
}

func TestInterpretSkoot_LightingFairlyLit(t *testing.T) {
	ev := InterpretSkoot(9, "30")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightFairlyLit {
		t.Errorf("expected LightFairlyLit for 30, got %d", *ev.Lighting)
	}
}

func TestInterpretSkoot_LightingSomewhatDark(t *testing.T) {
	ev := InterpretSkoot(9, "15")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightSomewhatDark {
		t.Errorf("expected LightSomewhatDark for 15, got %d", *ev.Lighting)
	}
}

func TestInterpretSkoot_LightingVeryDark(t *testing.T) {
	ev := InterpretSkoot(9, "4")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightVeryDark {
		t.Errorf("expected LightVeryDark for 4, got %d", *ev.Lighting)
	}
}

func TestInterpretSkoot_LightingExtremelyDark(t *testing.T) {
	ev := InterpretSkoot(9, "2")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightExtremelyDark {
		t.Errorf("expected LightExtremelyDark for 2, got %d", *ev.Lighting)
	}
}

func TestInterpretSkoot_LightingPitchBlack(t *testing.T) {
	ev := InterpretSkoot(9, "0")
	if ev == nil || ev.Lighting == nil {
		t.Fatal("expected lighting event")
	}
	if *ev.Lighting != types.LightPitchBlack {
		t.Errorf("expected LightPitchBlack for 0, got %d", *ev.Lighting)
	}
}

// --- Minimap rooms (channel 6) ---

func TestInterpretSkoot_MinimapRooms(t *testing.T) {
	ev := InterpretSkoot(6, "0,0,10,#ff0000,19.56,0,-10,10,#ffffff,37.8")
	if ev == nil {
		t.Fatal("expected non-nil event for minimap rooms")
	}
	if len(ev.Rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(ev.Rooms))
	}

	// First room: player room (red)
	r0 := ev.Rooms[0]
	if r0.X != 0 || r0.Y != 0 {
		t.Errorf("room 0 position: got (%d,%d), want (0,0)", r0.X, r0.Y)
	}
	if r0.Size != 10 {
		t.Errorf("room 0 size: got %d, want 10", r0.Size)
	}
	if r0.Color != "#ff0000" {
		t.Errorf("room 0 color: got %q, want #ff0000", r0.Color)
	}
	if r0.Brightness != 19.56 {
		t.Errorf("room 0 brightness: got %f, want 19.56", r0.Brightness)
	}

	// Second room: other room (white)
	r1 := ev.Rooms[1]
	if r1.X != 0 || r1.Y != -10 {
		t.Errorf("room 1 position: got (%d,%d), want (0,-10)", r1.X, r1.Y)
	}
	if r1.Color != "#ffffff" {
		t.Errorf("room 1 color: got %q, want #ffffff", r1.Color)
	}
}

func TestInterpretSkoot_MinimapRoomsInvalid(t *testing.T) {
	// Not a multiple of 5
	ev := InterpretSkoot(6, "0,0,10,#ff0000")
	if ev != nil {
		t.Error("expected nil for incomplete room data")
	}
}

// --- Minimap walls (channel 10) ---

func TestInterpretSkoot_MinimapWalls(t *testing.T) {
	ev := InterpretSkoot(10, "5,10,ver,0,5,-1,ver,1")
	if ev == nil {
		t.Fatal("expected non-nil event for minimap walls")
	}
	if len(ev.Walls) != 2 {
		t.Fatalf("expected 2 walls, got %d", len(ev.Walls))
	}

	// First wall: value 0 = blocked (not accessible)
	w0 := ev.Walls[0]
	if w0.X != 5 || w0.Y != 10 {
		t.Errorf("wall 0 position: got (%d,%d), want (5,10)", w0.X, w0.Y)
	}
	if w0.Type != "ver" {
		t.Errorf("wall 0 type: got %q, want ver", w0.Type)
	}
	if w0.Passable {
		t.Error("wall 0 value=0 should be blocked (not passable)")
	}

	// Second wall: value 1 = accessible (passable)
	w1 := ev.Walls[1]
	if w1.X != 5 || w1.Y != -1 {
		t.Errorf("wall 1 position: got (%d,%d), want (5,-1)", w1.X, w1.Y)
	}
	if w1.Type != "ver" {
		t.Errorf("wall 1 type: got %q, want ver", w1.Type)
	}
	if !w1.Passable {
		t.Error("wall 1 value=1 should be accessible (passable)")
	}
}

func TestInterpretSkoot_MinimapWallsInvalid(t *testing.T) {
	// Not a multiple of 4
	ev := InterpretSkoot(10, "5,10,ver")
	if ev != nil {
		t.Error("expected nil for incomplete wall data")
	}
}

// --- Unknown channel ---

func TestInterpretSkoot_UnknownChannel(t *testing.T) {
	ev := InterpretSkoot(99, "some random data")
	if ev != nil {
		t.Error("unknown channel should return nil")
	}
}
