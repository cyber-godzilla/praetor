package ui

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func excludeRule(pattern string) TabRule {
	return TabRule{
		Pattern: pattern,
		Regex:   compileWildcardPattern(pattern),
		Include: false,
		Active:  true,
	}
}

func includeRule(pattern string) TabRule {
	return TabRule{
		Pattern: pattern,
		Regex:   compileWildcardPattern(pattern),
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
	routed := RouteText(tabs, segs, "look", true)

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
	routed := RouteText(tabs, segs, "look", true)

	if routed&(1<<1) == 0 {
		t.Error("echo should route to exclude-only tab when EchoCommands=true")
	}
}

func TestRouteText_IncludeRulePresent_EchoFlagIgnored(t *testing.T) {
	// Tab with an include rule matching "look"; EchoCommands=false must not
	// prevent the echo from reaching the tab since include rules govern.
	tabs := buildTestTabs([]TabRule{includeRule("look")}, false)
	segs := []types.StyledSegment{{Text: "look"}}
	routed := RouteText(tabs, segs, "look", true)

	if routed&(1<<1) == 0 {
		t.Error("echo matching include rule should route regardless of EchoCommands flag")
	}
}

func TestRouteText_NormalText_ExcludeOnlyTab_Unaffected(t *testing.T) {
	// Non-echo text (isEcho=false) must route to exclude-only tab regardless
	// of EchoCommands flag.
	tabs := buildTestTabs([]TabRule{excludeRule("spam")}, false)
	segs := []types.StyledSegment{{Text: "look"}}
	routed := RouteText(tabs, segs, "look", false)

	if routed&(1<<1) == 0 {
		t.Error("normal text should route to exclude-only tab")
	}
}
