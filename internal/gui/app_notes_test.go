package gui

import (
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/notes"
)

func newNotesApp(t *testing.T) *GuiApp {
	t.Helper()
	deps := &Deps{
		Config: config.Defaults(),
		Notes:  notes.New(filepath.Join(t.TempDir(), "notes")),
	}
	return NewGuiApp(deps, &captureEmitter{})
}

func TestFacadeNotes_RoundTrip(t *testing.T) {
	a := newNotesApp(t)
	if err := a.SaveNote("", "Trade Runs", "buy low"); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	got, err := a.GetNote("trade runs")
	if err != nil || got.Body != "buy low" {
		t.Fatalf("GetNote: %+v err=%v", got, err)
	}
	list, err := a.ListNotes()
	if err != nil || len(list) != 1 || list[0].Title != "Trade Runs" {
		t.Fatalf("ListNotes: %+v err=%v", list, err)
	}
}

func TestFacadeNotes_ErrorsSurface(t *testing.T) {
	a := newNotesApp(t)
	if _, err := a.GetNote("nope"); err == nil {
		t.Error("GetNote on missing title should error")
	}
	if err := a.DeleteNote("nope"); err == nil {
		t.Error("DeleteNote on missing title should error")
	}
	_ = a.SaveNote("", "Dup", "x")
	if err := a.SaveNote("", "dup", "y"); err == nil {
		t.Error("duplicate title should error")
	}
}
