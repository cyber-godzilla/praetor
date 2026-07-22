package gui

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
)

type fakeDialogs struct {
	ret  string
	err  error
	seen struct {
		title, defaultDir string
	}
}

func (f *fakeDialogs) PickDirectory(title, defaultDir string) (string, error) {
	f.seen.title = title
	f.seen.defaultDir = defaultDir
	return f.ret, f.err
}

func TestPickScriptDir_ReturnsChosenPath(t *testing.T) {
	fd := &fakeDialogs{ret: "/home/user/scripts"}
	a := NewGuiApp(&Deps{Config: config.Defaults(), Dialogs: fd}, &captureEmitter{})

	got, err := a.PickScriptDir()
	if err != nil {
		t.Fatalf("PickScriptDir: %v", err)
	}
	if got != "/home/user/scripts" {
		t.Errorf("PickScriptDir = %q, want /home/user/scripts", got)
	}
}

func TestPickScriptDir_NilBackendReturnsEmpty(t *testing.T) {
	a := NewGuiApp(&Deps{Config: config.Defaults()}, &captureEmitter{})
	got, err := a.PickScriptDir()
	if err != nil || got != "" {
		t.Fatalf("PickScriptDir with nil Dialogs = (%q, %v), want (\"\", nil)", got, err)
	}
}
