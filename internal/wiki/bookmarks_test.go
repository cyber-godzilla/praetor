package wiki

import (
	"strings"
	"testing"
)

func TestSectionsNonEmpty(t *testing.T) {
	if len(sections) == 0 {
		t.Fatal("expected at least one section")
	}
	for _, s := range sections {
		if s.Name == "" {
			t.Errorf("section with empty name: %+v", s)
		}
		if len(s.Bookmarks) == 0 {
			t.Errorf("section %q has no bookmarks", s.Name)
		}
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	if slug, ok := Lookup("STATS"); !ok || slug != "stats" {
		t.Errorf("Lookup(STATS) = (%q, %v), want (stats, true)", slug, ok)
	}
}

func TestLookup_HyphenSpaceTolerant(t *testing.T) {
	if slug, ok := Lookup("healing guide"); !ok || slug != "healing-guide" {
		t.Errorf("Lookup(healing guide) = (%q, %v), want (healing-guide, true)", slug, ok)
	}
}

func TestLookup_Miss(t *testing.T) {
	if _, ok := Lookup("zzz-not-a-real-key-xyz"); ok {
		t.Error("Lookup of nonsense key should return ok=false")
	}
}

func TestKeysSorted(t *testing.T) {
	keys := Keys()
	for i := 1; i < len(keys); i++ {
		if strings.Compare(keys[i-1], keys[i]) > 0 {
			t.Errorf("Keys not sorted: %q > %q", keys[i-1], keys[i])
		}
	}
}

func TestKeysCount(t *testing.T) {
	keys := Keys()
	if len(keys) < 5 {
		t.Errorf("expected at least a few bookmarks, got %d", len(keys))
	}
}

func TestURL(t *testing.T) {
	got := URL("stats")
	want := "http://eternal-city.wikidot.com/stats"
	if got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}
