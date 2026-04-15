package ui

import (
	"regexp"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// TabKind identifies the type of tab.
type TabKind int

const (
	TabKindAll     TabKind = iota // shows all text
	TabKindMetrics                // metrics dashboard
	TabKindDebug                  // debug SKOOT data
	TabKindCustom                 // user-defined with match rules
)

// TabDef defines a tab in the dynamic tab system.
type TabDef struct {
	Name    string
	Kind    TabKind
	Visible bool
	Rules   []TabRule
	Pane    OutputPane
}

// TabRule is a compiled match rule for custom tabs.
type TabRule struct {
	Pattern string // original pattern with wildcards
	Regex   *regexp.Regexp
	Include bool // true = "does match" (include lines matching), false = "does not match" (include lines NOT matching)
	Active  bool
}

// compileWildcardPattern converts a wildcard pattern (* and ?) to a regexp.
// Patterns are substring matches by default — no anchoring.
// * matches any characters, ? matches a single character.
func compileWildcardPattern(pattern string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)
	// No ^ or $ anchors — substring match.
	re, err := regexp.Compile("(?i)" + escaped)
	if err != nil {
		return regexp.MustCompile("(?i)" + regexp.QuoteMeta(pattern))
	}
	return re
}

// BuildTabs creates the dynamic tab list from config.
func BuildTabs(scrollback int, debugMode bool, customTabs []config.CustomTabConfig) []TabDef {
	var tabs []TabDef

	// All tab — always first, always visible.
	tabs = append(tabs, TabDef{
		Name:    "All",
		Kind:    TabKindAll,
		Visible: true,
		Pane:    NewOutputPane(scrollback),
	})

	// Custom tabs from config.
	for _, ct := range customTabs {
		var rules []TabRule
		for _, r := range ct.Rules {
			rules = append(rules, TabRule{
				Pattern: r.Pattern,
				Regex:   compileWildcardPattern(r.Pattern),
				Include: r.Include,
				Active:  r.Active,
			})
		}
		tabs = append(tabs, TabDef{
			Name:    ct.Name,
			Kind:    TabKindCustom,
			Visible: ct.Visible,
			Rules:   rules,
			Pane:    NewOutputPane(scrollback),
		})
	}

	// Metrics tab.
	tabs = append(tabs, TabDef{
		Name:    "Metrics",
		Kind:    TabKindMetrics,
		Visible: true,
	})

	// Debug tab.
	tabs = append(tabs, TabDef{
		Name:    "Debug",
		Kind:    TabKindDebug,
		Visible: debugMode,
	})

	return tabs
}

// MatchesTab returns true if the text should appear in a custom tab
// based on its rules.
func MatchesTab(text string, rules []TabRule) bool {
	hasIncludes := false
	matched := false

	for _, r := range rules {
		if !r.Active {
			continue
		}

		if r.Include {
			hasIncludes = true
			if r.Regex.MatchString(text) {
				matched = true
			}
		} else {
			// "Does not match" — if text matches the exclusion pattern, reject.
			if r.Regex.MatchString(text) {
				return false
			}
		}
	}

	// If there are no include rules, accept everything not excluded.
	if !hasIncludes {
		return true
	}

	return matched
}

// RouteText sends styled segments to all matching custom tabs.
// Returns a bitmask of tab indices that received text.
func RouteText(tabs []TabDef, segments []types.StyledSegment, plainText string) uint64 {
	var routed uint64
	for i := range tabs {
		switch tabs[i].Kind {
		case TabKindAll:
			tabs[i].Pane.Append(segments)
			routed |= 1 << uint(i)
		case TabKindCustom:
			if tabs[i].Visible && MatchesTab(plainText, tabs[i].Rules) {
				tabs[i].Pane.Append(segments)
				routed |= 1 << uint(i)
			}
		}
	}
	return routed
}

// TabsToConfig converts the current tab state back to config for saving.
func TabsToConfig(tabs []TabDef) []config.CustomTabConfig {
	var result []config.CustomTabConfig
	for _, t := range tabs {
		if t.Kind != TabKindCustom {
			continue
		}
		ct := config.CustomTabConfig{
			Name:    t.Name,
			Visible: t.Visible,
		}
		for _, r := range t.Rules {
			ct.Rules = append(ct.Rules, config.TabRuleConfig{
				Pattern: r.Pattern,
				Include: r.Include,
				Active:  r.Active,
			})
		}
		result = append(result, ct)
	}
	return result
}
