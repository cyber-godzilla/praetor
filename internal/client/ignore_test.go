package client

import "testing"

func TestIgnoreFilter_OOC_CanonicalExample(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"xXSephirothXx"})
	line := `<8:14 pm OOC> xXSephirothXx says, "lol"`
	if !f.ShouldDrop(line) {
		t.Errorf("expected drop for %q", line)
	}
}

func TestIgnoreFilter_OOC_CapitalMeridian(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"dArKwInG666"})
	line := `<10:05 PM OOC> dArKwInG666 says, "anyone on?"`
	if !f.ShouldDrop(line) {
		t.Errorf("expected drop for %q", line)
	}
}

func TestIgnoreFilter_Think_CanonicalExample(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetThink([]string{"Travis"})
	line := `<Travis thinks aloud: But why, though?>`
	if !f.ShouldDrop(line) {
		t.Errorf("expected drop for %q", line)
	}
}

func TestIgnoreFilter_CaseInsensitiveLookup(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"EmoCryBaby"})
	line := `<8:14 pm OOC> EMOCRYBABY says, "edgy"`
	if !f.ShouldDrop(line) {
		t.Errorf("expected case-insensitive drop for %q", line)
	}
}

func TestIgnoreFilter_ListsSeparate(t *testing.T) {
	// Same string is on the OOC list. A think-aloud from a character
	// with that same name must NOT be dropped.
	f := NewIgnoreFilter()
	f.SetOOC([]string{"Andrea"})
	thinkLine := `<Andrea thinks aloud: a thought>`
	if f.ShouldDrop(thinkLine) {
		t.Errorf("OOC list must not affect think-aloud lines: %q dropped unexpectedly", thinkLine)
	}
}

func TestIgnoreFilter_EmptyListsNeverDrop(t *testing.T) {
	f := NewIgnoreFilter()
	if f.ShouldDrop(`<8:14 pm OOC> xXSephirothXx says, "hi"`) {
		t.Error("empty OOC list should not drop OOC lines")
	}
	if f.ShouldDrop(`<Travis thinks aloud: hi>`) {
		t.Error("empty Think list should not drop think lines")
	}
}

func TestIgnoreFilter_NonMatchingPrefixNeverDrops(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"xXSephirothXx"})
	f.SetThink([]string{"Travis"})
	cases := []string{
		`Travis says, "hello"`,                   // plain say
		`Travis appears at the entrance.`,        // narrative
		`You feel a chill.`,                      // ambient
		`<8:14 pm OOC> Tobias says, "different"`, // OOC by an unlisted account
		`<Andrea thinks aloud: not blocked>`,     // think by an unlisted character
	}
	for _, c := range cases {
		if f.ShouldDrop(c) {
			t.Errorf("non-matching line dropped: %q", c)
		}
	}
}

func TestIgnoreFilter_SetReplacesEntries(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"M0rt1c1aNvOiD"})
	f.SetOOC([]string{"MasterChief1337"})
	if f.ShouldDrop(`<8:14 pm OOC> M0rt1c1aNvOiD says, "still here?"`) {
		t.Error("Set should fully replace the previous list, but old entry still drops")
	}
	if !f.ShouldDrop(`<8:14 pm OOC> MasterChief1337 says, "haxxor"`) {
		t.Error("new entry not honored after Set replacement")
	}
}
