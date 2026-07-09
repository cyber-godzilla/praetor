package gui

import (
	"errors"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
)

type fakeClipboard struct {
	text   string
	getErr error
}

func (f *fakeClipboard) GetText() (string, error)  { return f.text, f.getErr }
func (f *fakeClipboard) SetText(text string) error { f.text = text; return nil }

func TestClipboardGetSetDelegates(t *testing.T) {
	fc := &fakeClipboard{}
	a := NewGuiApp(&Deps{Config: config.Defaults(), Clipboard: fc}, &captureEmitter{})

	if err := a.ClipboardSet("hello"); err != nil {
		t.Fatalf("ClipboardSet: %v", err)
	}
	if fc.text != "hello" {
		t.Fatalf("SetText not delegated: got %q", fc.text)
	}
	got, err := a.ClipboardGet()
	if err != nil {
		t.Fatalf("ClipboardGet: %v", err)
	}
	if got != "hello" {
		t.Fatalf("ClipboardGet = %q, want hello", got)
	}
}

func TestClipboardGetPropagatesError(t *testing.T) {
	fc := &fakeClipboard{getErr: errors.New("boom")}
	a := NewGuiApp(&Deps{Config: config.Defaults(), Clipboard: fc}, &captureEmitter{})
	if _, err := a.ClipboardGet(); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestClipboardNilBackendIsSafe(t *testing.T) {
	a := NewGuiApp(&Deps{Config: config.Defaults()}, &captureEmitter{})
	got, err := a.ClipboardGet()
	if err != nil || got != "" {
		t.Fatalf("nil-backend get = %q, %v; want \"\", nil", got, err)
	}
	if err := a.ClipboardSet("x"); err != nil {
		t.Fatalf("nil-backend set = %v; want nil", err)
	}
}
