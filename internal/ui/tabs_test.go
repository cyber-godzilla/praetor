package ui

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestCustomViewIDSurvivesTUIBuildAndSave(t *testing.T) {
	built := BuildTabs(100, false, []config.CustomTabConfig{{
		ID: "custom-chat", Name: "Chat", Visible: true,
	}})
	got := TabsToConfig(built)
	if len(got) != 1 || got[0].ID != "custom-chat" {
		t.Fatalf("custom view identity did not round trip: %+v", got)
	}
}

func excludeRule(pattern string) TabRule {
	return TabRule{
		Pattern: pattern,
		Include: false,
		Active:  true,
	}
}

func includeRule(pattern string) TabRule {
	return TabRule{
		Pattern: pattern,
		Include: true,
		Active:  true,
	}
}

// buildTestTabs returns [All, Custom] with the given rules + echo flag.
func buildTestTabs(rules []TabRule, echoCommands bool) []TabDef {
	return []TabDef{
		{Name: "All", Kind: TabKindAll, Visible: true, Pane: NewOutputPane(100)},
		{Name: "Custom", Kind: TabKindCustom, Visible: true, Rules: rules, EchoCommands: echoCommands, Pane: NewOutputPane(100)},
	}
}

func TestRouteText_ExcludeOnlyTab_EchoSkippedByDefault(t *testing.T) {
	tabs := buildTestTabs([]TabRule{excludeRule("spam")}, false)
	segs := []types.StyledSegment{{Text: "look"}}
	routed := RouteText(tabs, segs, "look", true, nil)

	// All tab = index 0 (bit 0). Custom tab = index 1 (bit 1).
	if routed&(1<<1) != 0 {
		t.Error("echo should not route to exclude-only tab when EchoCommands=false")
	}
	if routed&(1<<0) == 0 {
		t.Error("echo should still route to All tab")
	}
}

func TestRouteText_ExcludeOnlyTab_EchoRoutedWhenEnabled(t *testing.T) {
	tabs := buildTestTabs([]TabRule{excludeRule("spam")}, true)
	segs := []types.StyledSegment{{Text: "look"}}
	routed := RouteText(tabs, segs, "look", true, nil)

	if routed&(1<<1) == 0 {
		t.Error("echo should route to exclude-only tab when EchoCommands=true")
	}
}

func TestRouteText_IncludeRulePresent_EchoFlagIgnored(t *testing.T) {
	// Tab with an include rule matching "look"; EchoCommands=false must not
	// prevent the echo from reaching the tab since include rules govern.
	tabs := buildTestTabs([]TabRule{includeRule("look")}, false)
	segs := []types.StyledSegment{{Text: "look"}}
	routed := RouteText(tabs, segs, "look", true, nil)

	if routed&(1<<1) == 0 {
		t.Error("echo matching include rule should route regardless of EchoCommands flag")
	}
}

func TestRouteText_NormalText_ExcludeOnlyTab_Unaffected(t *testing.T) {
	// Non-echo text (isEcho=false) must route to exclude-only tab regardless
	// of EchoCommands flag.
	tabs := buildTestTabs([]TabRule{excludeRule("spam")}, false)
	segs := []types.StyledSegment{{Text: "look"}}
	routed := RouteText(tabs, segs, "look", false, nil)

	if routed&(1<<1) == 0 {
		t.Error("normal text should route to exclude-only tab")
	}
}

func TestMatchesTab_UsesLiveVariablesAndSharedWildcardEscapes(t *testing.T) {
	rules := []TabRule{{Pattern: `${target} dropped \*`, Include: true, Active: true}}
	if !MatchesTab("Apula dropped *", rules, map[string]string{"target": "Apula"}) {
		t.Fatal("expanded escaped-wildcard rule did not match")
	}
	if MatchesTab("Apula dropped sword", rules, map[string]string{"target": "Apula"}) {
		t.Fatal("escaped star behaved as a wildcard")
	}
	if MatchesTab("Apula dropped *", rules, map[string]string{"target": "Constantine"}) {
		t.Fatal("rule did not observe the current variable snapshot")
	}
}

func TestMatchesTab_UnresolvedExclusionMatchesNothing(t *testing.T) {
	rules := []TabRule{{Pattern: "${missing}", Include: false, Active: true}}
	if !MatchesTab("anything", rules, nil) {
		t.Fatal("unresolved exclusion accidentally excluded text")
	}
}

func TestHandleEvent_MenuOpenDefersWithoutDroppingBatchText(t *testing.T) {
	a := App{
		tabs:      buildTestTabs(nil, false),
		activeTab: 0,
		unread:    []bool{false, false},
	}
	msg := EventMsg{Events: []types.Event{
		types.GameTextEvent{Text: "before menu", Styled: plainSegment("before menu")},
		types.WikiOpenMenuEvent{},
		types.GameTextEvent{Text: "after menu", Styled: plainSegment("after menu")},
	}}

	model, _ := a.handleEvent(msg)

	// Both texts must reach the All pane — the text queued after the menu event
	// in the same batch was previously discarded by the mid-loop return.
	if got := len(a.tabs[0].Pane.lines); got != 2 {
		t.Fatalf("All pane has %d lines, want 2 (text after the menu event was dropped)", got)
	}
	// The deferred menu still opens.
	if got := model.(App).state; got != stateWikiMenu {
		t.Errorf("state = %v, want stateWikiMenu (menu was not opened)", got)
	}
}
