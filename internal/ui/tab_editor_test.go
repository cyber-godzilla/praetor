package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber-godzilla/praetor/internal/config"
)

func newEditorOnTab(rules []config.TabRuleConfig, echo bool) TabEditor {
	te := NewTabEditor([]config.CustomTabConfig{{
		Name:         "T1",
		Visible:      true,
		EchoCommands: echo,
		Rules:        rules,
	}})
	te.SetSize(80, 24)
	te.editIdx = 0
	te.mode = temEdit
	return te
}

func TestTabEditor_EKeyTogglesEchoCommands_ExcludeOnly(t *testing.T) {
	te := newEditorOnTab([]config.TabRuleConfig{
		{Pattern: "spam", Include: false, Active: true},
	}, false)

	te, _ = te.Update(runeMsg('e'))
	if !te.tabs[0].EchoCommands {
		t.Error("expected EchoCommands=true after 'e' on exclude-only tab")
	}
	te, _ = te.Update(runeMsg('e'))
	if te.tabs[0].EchoCommands {
		t.Error("expected EchoCommands=false after second 'e' toggle")
	}
}

func TestTabEditor_EKeyIgnoredWhenIncludeRuleActive(t *testing.T) {
	te := newEditorOnTab([]config.TabRuleConfig{
		{Pattern: "loot", Include: true, Active: true},
	}, false)

	te, _ = te.Update(runeMsg('e'))
	if te.tabs[0].EchoCommands {
		t.Error("expected 'e' to be ignored when tab has active include rule")
	}
}

func TestTabEditor_EchoesRowRendered_ExcludeOnly(t *testing.T) {
	te := newEditorOnTab([]config.TabRuleConfig{
		{Pattern: "spam", Include: false, Active: true},
	}, false)
	if !strings.Contains(te.View(), "Echoes:") {
		t.Error("exclude-only tab edit view should contain Echoes: row")
	}
}

func TestTabEditor_EchoesRowHidden_IncludeRulePresent(t *testing.T) {
	te := newEditorOnTab([]config.TabRuleConfig{
		{Pattern: "loot", Include: true, Active: true},
	}, false)
	if strings.Contains(te.View(), "Echoes:") {
		t.Error("tab with include rule should not render Echoes: row")
	}
}

func TestTabEditor_EchoesRowHidden_ZeroRules(t *testing.T) {
	// Zero-rule tab counts as exclude-only (catch-all). Row should be visible.
	te := newEditorOnTab(nil, false)
	if !strings.Contains(te.View(), "Echoes:") {
		t.Error("zero-rule tab should render Echoes: row (exclude-only catch-all)")
	}
}

func TestTabEditor_EchoesRowShowsState(t *testing.T) {
	te := newEditorOnTab([]config.TabRuleConfig{
		{Pattern: "spam", Include: false, Active: true},
	}, true)
	v := te.View()
	if !strings.Contains(v, "Echoes: ON") {
		t.Errorf("expected 'Echoes: ON' in view, got: %s", v)
	}
}

// silence unused import when running subsets
var _ = tea.KeyMsg{}
