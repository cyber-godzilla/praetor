package ui

import (
	"testing"
)

// newTabBarTestApp builds a minimal App with just the fields renderTabBar
// reads. The full NewApp constructor takes 27 arguments and pulls in
// keyring/config, neither of which is relevant to render-cache behavior.
func newTabBarTestApp(tabs []TabDef, activeTab int, unread []bool) App {
	return App{
		width:     80,
		tabs:      tabs,
		activeTab: activeTab,
		unread:    unread,
		tabBar:    &tabBarCache{},
	}
}

func TestRenderTabBar_CachedAcrossNoOpCalls(t *testing.T) {
	a := newTabBarTestApp(
		[]TabDef{
			{Name: "All", Visible: true},
			{Name: "Combat", Visible: true},
		},
		0,
		[]bool{false, false},
	)

	v1 := a.renderTabBar()
	if v1 == "" {
		t.Fatal("renderTabBar should return non-empty string")
	}
	v2 := a.renderTabBar()
	if v1 != v2 {
		t.Fatal("renderTabBar should return byte-identical result on repeat call")
	}
}

func TestRenderTabBar_ActiveTabChangeInvalidates(t *testing.T) {
	// Note: in go test there's no TTY, so lipgloss strips color escapes
	// — active vs inactive labels render as identical bytes. We assert
	// the cached key was updated, which is what the cache actually
	// turns on at runtime when color is present.
	a := newTabBarTestApp(
		[]TabDef{
			{Name: "All", Visible: true},
			{Name: "Combat", Visible: true},
		},
		0,
		[]bool{false, false},
	)
	_ = a.renderTabBar()
	if a.tabBar.activeTab != 0 {
		t.Fatalf("cache should have stored activeTab=0; got %d", a.tabBar.activeTab)
	}
	a.activeTab = 1
	_ = a.renderTabBar()
	if a.tabBar.activeTab != 1 {
		t.Fatal("activeTab change should be reflected in the cache key after re-render")
	}
}

func TestRenderTabBar_UnreadChangeInvalidates(t *testing.T) {
	a := newTabBarTestApp(
		[]TabDef{
			{Name: "All", Visible: true},
			{Name: "Combat", Visible: true},
		},
		0,
		[]bool{false, false},
	)
	_ = a.renderTabBar()
	baselineMask := a.tabBar.unreadMask
	a.unread[1] = true
	_ = a.renderTabBar()
	if a.tabBar.unreadMask == baselineMask {
		t.Fatal("unread bit flip should change cached unreadMask")
	}
	if a.tabBar.unreadMask&(1<<1) == 0 {
		t.Fatalf("expected unreadMask bit 1 set; got %#x", a.tabBar.unreadMask)
	}
}

func TestRenderTabBar_WidthChangeInvalidates(t *testing.T) {
	a := newTabBarTestApp(
		[]TabDef{
			{Name: "All", Visible: true},
		},
		0,
		[]bool{false},
	)
	before := a.renderTabBar()
	a.width = 60
	after := a.renderTabBar()
	if before == after {
		t.Fatal("width change should invalidate cache")
	}
}

func TestRenderTabBar_VisibilityChangeInvalidates(t *testing.T) {
	a := newTabBarTestApp(
		[]TabDef{
			{Name: "All", Visible: true},
			{Name: "Debug", Visible: false},
		},
		0,
		[]bool{false, false},
	)
	before := a.renderTabBar()
	a.tabs[1].Visible = true
	after := a.renderTabBar()
	if before == after {
		t.Fatal("tab visibility change should invalidate cache")
	}
}

func TestRenderTabBar_NameChangeInvalidates(t *testing.T) {
	a := newTabBarTestApp(
		[]TabDef{
			{Name: "All", Visible: true},
			{Name: "Custom", Visible: true},
		},
		0,
		[]bool{false, false},
	)
	before := a.renderTabBar()
	a.tabs[1].Name = "Renamed"
	after := a.renderTabBar()
	if before == after {
		t.Fatal("tab name change should invalidate cache")
	}
}

func TestVisibleWidth_PlainASCII(t *testing.T) {
	cases := map[string]int{
		"":       0,
		"hello":  5,
		"a b c":  5,
		" lead":  5, // padLines preserves leading whitespace
		"trail ": 6,
	}
	for in, want := range cases {
		if got := visibleWidth(in); got != want {
			t.Errorf("visibleWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestVisibleWidth_StripsANSIEscapes(t *testing.T) {
	// SGR color sequences should contribute zero to visible width.
	styled := "\x1b[31mhello\x1b[0m"
	if got := visibleWidth(styled); got != 5 {
		t.Errorf("visibleWidth(red 'hello') = %d, want 5", got)
	}
	// Reset + bold + underline + content + reset.
	combo := "\x1b[0m\x1b[1m\x1b[4mtext\x1b[0m"
	if got := visibleWidth(combo); got != 4 {
		t.Errorf("visibleWidth(complex SGR) = %d, want 4", got)
	}
	// CSI cursor positioning escapes (non-SGR terminator).
	cursor := "\x1b[2K\x1b[1;1Habcd"
	if got := visibleWidth(cursor); got != 4 {
		t.Errorf("visibleWidth(CSI + 'abcd') = %d, want 4", got)
	}
}

func TestVisibleWidth_OSCEscape(t *testing.T) {
	// OSC sequence terminated by BEL.
	bel := "\x1b]0;title\x07hello"
	if got := visibleWidth(bel); got != 5 {
		t.Errorf("visibleWidth(OSC BEL + 'hello') = %d, want 5", got)
	}
	// OSC sequence terminated by ESC\.
	stesc := "\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\"
	if got := visibleWidth(stesc); got != 4 {
		t.Errorf("visibleWidth(OSC8 link) = %d, want 4", got)
	}
}

func TestVisibleWidth_MultibyteRunes(t *testing.T) {
	// Praetor uses these glyphs in real output: HR rule, lighting icons,
	// compass arrows. All single-cell visually; should count as 1 each.
	cases := map[string]int{
		"───":        3,
		"☀ Bright":   8, // sun + space + 6 letters
		"◐ Dim":      5,
		"compass: ↑": 10,
	}
	for in, want := range cases {
		if got := visibleWidth(in); got != want {
			t.Errorf("visibleWidth(%q) = %d, want %d", in, got, want)
		}
	}
}
