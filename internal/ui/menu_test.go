package ui

import (
	"strings"
	"testing"
)

func TestMenu_HidesQuickCycleWhenNoModesAvailable(t *testing.T) {
	m := NewMenu(false, false, false, false, false, false, "", false)
	for _, item := range m.items {
		if item.label == "Quick-Cycle Modes" {
			t.Error("Quick-Cycle Modes should be hidden when modesAvailable=false")
		}
	}
}

func TestMenu_ShowsQuickCycleWhenModesAvailable(t *testing.T) {
	m := NewMenu(false, false, false, false, false, false, "", true)
	found := false
	for _, item := range m.items {
		if item.label == "Quick-Cycle Modes" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Quick-Cycle Modes should be visible when modesAvailable=true")
	}
}

// Sanity: the ScriptDirs item is unconditional and must remain.
func TestMenu_AlwaysShowsScriptDirectories(t *testing.T) {
	for _, available := range []bool{false, true} {
		m := NewMenu(false, false, false, false, false, false, "", available)
		found := false
		for _, item := range m.items {
			if strings.Contains(item.label, "Script Directories") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Script Directories must always be present (modesAvailable=%v)", available)
		}
	}
}
