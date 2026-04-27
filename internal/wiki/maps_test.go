package wiki

import "testing"

func TestMapSectionsNonEmpty(t *testing.T) {
	if len(mapSections) == 0 {
		t.Fatal("expected at least one map section")
	}
	for _, s := range mapSections {
		if s.Name == "" {
			t.Errorf("section with empty name: %+v", s)
		}
		if len(s.Bookmarks) == 0 {
			t.Errorf("section %q has no bookmarks", s.Name)
		}
	}
}

func TestLookupMap_HitAndMiss(t *testing.T) {
	if slug, ok := LookupMap("monlon"); !ok || slug != "monlon" {
		t.Errorf("LookupMap(monlon) = (%q, %v), want (monlon, true)", slug, ok)
	}
	if slug, ok := LookupMap("MONLON"); !ok || slug != "monlon" {
		t.Errorf("LookupMap(MONLON) = (%q, %v), want (monlon, true)", slug, ok)
	}
	if _, ok := LookupMap("zzz-not-real-xyz"); ok {
		t.Error("LookupMap on nonsense should return false")
	}
}

func TestLookupMap_AliasesPointSameSlug(t *testing.T) {
	a, _ := LookupMap("gardens")
	b, _ := LookupMap("hospice")
	if a == "" || a != b {
		t.Errorf("gardens=%q hospice=%q — should both resolve to gardens-and-hospice", a, b)
	}
}

func TestLookupMap_PreservesColonInSlug(t *testing.T) {
	slug, ok := LookupMap("monlon-ravines")
	if !ok || slug != "hg:monlon-ravines" {
		t.Errorf("LookupMap(monlon-ravines) = (%q, %v), want (hg:monlon-ravines, true)", slug, ok)
	}
}

func TestMapURLWithColonSlug(t *testing.T) {
	url := URL("hg:monlon-ravines")
	want := "http://eternal-city.wikidot.com/hg:monlon-ravines"
	if url != want {
		t.Errorf("URL = %q, want %q", url, want)
	}
}

func TestMapKeysSorted(t *testing.T) {
	keys := MapKeys()
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("MapKeys not sorted: %q > %q", keys[i-1], keys[i])
		}
	}
}
