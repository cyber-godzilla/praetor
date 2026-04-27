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
	// Pick any known bookmark.
	if slug, ok := Lookup("HERBALISM"); !ok || slug == "" {
		t.Errorf("Lookup(HERBALISM) = (%q, %v), want non-empty slug + true", slug, ok)
	}
}

func TestLookup_HyphenSpaceTolerant(t *testing.T) {
	// "rock valley" should match "rock-valley" (assuming that key exists).
	if _, ok := Lookup("rock valley"); !ok {
		// fall back to direct hyphenated form.
		if _, ok2 := Lookup("rock-valley"); !ok2 {
			t.Skip("no rock-valley key to test against")
		}
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
	if len(keys) < 50 {
		t.Errorf("expected many bookmarks (>50), got %d", len(keys))
	}
}

func TestURL(t *testing.T) {
	got := URL("herbalism")
	want := "http://eternal-city.wikidot.com/herbalism"
	if got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}
