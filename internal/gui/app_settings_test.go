package gui

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
)

// Wails dispatches each bound setter on its own goroutine, so two settings
// changes can overlap: one mutating a config field while another marshals the
// whole struct in Save. Serializing mutate+save under the facade lock keeps that
// race-free. Run with -race.
func TestFacadeSettings_ConcurrentSettersDoNotRace(t *testing.T) {
	deps := &Deps{
		Config:     config.Defaults(),
		ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
	}
	a := NewGuiApp(deps, &captureEmitter{})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_ = a.SetHideIPs(i%4 == 0)
			} else {
				_ = a.SetOutputFontSize(12 + i)
			}
		}(i)
	}
	wg.Wait()

	if _, err := config.Load(deps.ConfigPath); err != nil {
		t.Fatalf("config unreadable after concurrent setters: %v", err)
	}
}
