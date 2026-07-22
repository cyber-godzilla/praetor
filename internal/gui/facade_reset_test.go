package gui

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func emittedKinds(em *captureEmitter) []string {
	var kinds []string
	for _, e := range em.snapshot() {
		if batch, ok := e.data.([]WireEvent); ok {
			for _, w := range batch {
				kinds = append(kinds, w.Kind)
			}
		}
	}
	return kinds
}

func hasKind(kinds []string, k string) bool {
	for _, got := range kinds {
		if got == k {
			return true
		}
	}
	return false
}

func skootRooms() types.SKOOTUpdateEvent {
	return types.SKOOTUpdateEvent{
		Rooms: []types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ffffff", Brightness: 20}},
	}
}

func TestFacade_DisconnectSkipsTrailingSKOOTInBatch(t *testing.T) {
	em := &captureEmitter{}
	a := NewGuiApp(&Deps{Config: config.Defaults()}, em)

	// Disconnect then a trailing SKOOT in the same batch: the reset must stick,
	// so no minimap is re-emitted.
	a.processBatch([]types.Event{
		types.DisconnectedEvent{Reason: "connection closed"},
		skootRooms(),
	})

	if hasKind(emittedKinds(em), KindMinimap) {
		t.Error("trailing SKOOT repopulated the minimap after a disconnect")
	}
}

func TestFacade_SKOOTBeforeDisconnectStillApplies(t *testing.T) {
	em := &captureEmitter{}
	a := NewGuiApp(&Deps{Config: config.Defaults()}, em)

	// SKOOT before the disconnect must apply normally (minimap emitted), then reset.
	a.processBatch([]types.Event{
		skootRooms(),
		types.DisconnectedEvent{Reason: "connection closed"},
	})

	if !hasKind(emittedKinds(em), KindMinimap) {
		t.Error("SKOOT before a disconnect was dropped; it should apply then reset")
	}
}
