package client

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/protocol"
)

func TestIgnoreFilter_OOC_CanonicalExample(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"xXSephirothXx"})
	line := `<8:14 pm OOC> xXSephirothXx says, "lol"`
	ch, name, hit := f.Match(line)
	if !hit {
		t.Fatalf("expected hit for %q", line)
	}
	if ch != IgnoreChannelOOC {
		t.Errorf("channel: got %v, want IgnoreChannelOOC", ch)
	}
	if name != "xXSephirothXx" {
		t.Errorf("name: got %q, want xXSephirothXx", name)
	}
}

func TestIgnoreFilter_OOC_CapitalMeridian(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"dArKwInG666"})
	line := `<10:05 PM OOC> dArKwInG666 says, "anyone on?"`
	ch, name, hit := f.Match(line)
	if !hit {
		t.Fatalf("expected hit for %q", line)
	}
	if ch != IgnoreChannelOOC {
		t.Errorf("channel: got %v, want IgnoreChannelOOC", ch)
	}
	if name != "dArKwInG666" {
		t.Errorf("name: got %q, want dArKwInG666", name)
	}
}

func TestIgnoreFilter_Think_CanonicalExample(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetThink([]string{"Travis"})
	line := `<Travis thinks aloud: But why, though?>`
	ch, name, hit := f.Match(line)
	if !hit {
		t.Fatalf("expected hit for %q", line)
	}
	if ch != IgnoreChannelThink {
		t.Errorf("channel: got %v, want IgnoreChannelThink", ch)
	}
	if name != "Travis" {
		t.Errorf("name: got %q, want Travis", name)
	}
}

func TestIgnoreFilter_CaseInsensitiveLookup(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"EmoCryBaby"})
	line := `<8:14 pm OOC> EMOCRYBABY says, "edgy"`
	ch, name, hit := f.Match(line)
	if !hit {
		t.Errorf("expected case-insensitive hit for %q", line)
	}
	if ch != IgnoreChannelOOC {
		t.Errorf("channel: got %v, want IgnoreChannelOOC", ch)
	}
	if name != "EMOCRYBABY" {
		// The captured name is case-preserved from the input.
		t.Errorf("name: got %q, want EMOCRYBABY", name)
	}
}

func TestIgnoreFilter_ListsSeparate(t *testing.T) {
	// Same string is on the OOC list. A think-aloud from a character
	// with that same name must NOT be matched.
	f := NewIgnoreFilter()
	f.SetOOC([]string{"Andrea"})
	thinkLine := `<Andrea thinks aloud: a thought>`
	if _, _, hit := f.Match(thinkLine); hit {
		t.Errorf("OOC list must not affect think-aloud lines: %q matched unexpectedly", thinkLine)
	}
}

func TestIgnoreFilter_EmptyListsNeverMatch(t *testing.T) {
	f := NewIgnoreFilter()
	if _, _, hit := f.Match(`<8:14 pm OOC> xXSephirothXx says, "hi"`); hit {
		t.Error("empty OOC list should not match OOC lines")
	}
	if _, _, hit := f.Match(`<Travis thinks aloud: hi>`); hit {
		t.Error("empty Think list should not match think lines")
	}
}

func TestIgnoreFilter_NonMatchingPrefixNeverMatches(t *testing.T) {
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
		if _, _, hit := f.Match(c); hit {
			t.Errorf("non-matching line matched: %q", c)
		}
	}
}

func TestIgnoreFilter_SetReplacesEntries(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"M0rt1c1aNvOiD"})
	f.SetOOC([]string{"MasterChief1337"})
	if _, _, hit := f.Match(`<8:14 pm OOC> M0rt1c1aNvOiD says, "still here?"`); hit {
		t.Error("Set should fully replace the previous list, but old entry still matches")
	}
	if _, _, hit := f.Match(`<8:14 pm OOC> MasterChief1337 says, "haxxor"`); !hit {
		t.Error("new entry not honored after Set replacement")
	}
}

// TestIgnoreFilter_IntegrationViaHTMLParse verifies the wired path:
// real (HTML-escaped) game text → ParseHTMLWithIndent → Match.
// Production probe of session logs confirmed the server sends HTML-
// escaped angle brackets (e.g. "&lt;8:14 pm OOC&gt; ..."), which the
// parser decodes to literal "<8:14 pm OOC> ...". Without this test,
// only the regex unit tests cover the contract — they don't prove the
// caller's actual input is what they expect.
func TestIgnoreFilter_IntegrationViaHTMLParse(t *testing.T) {
	f := NewIgnoreFilter()
	f.SetOOC([]string{"xXSephirothXx", "dArKwInG666"})
	f.SetThink([]string{"Travis", "Tobias"})

	// Inputs as they arrive on the wire (HTML-escaped + optional
	// <font> wrapper). The parser must decode the entities and strip
	// the font tag before our regex sees the text.
	drops := []string{
		`&lt;8:14 pm OOC&gt; xXSephirothXx says, "lfg anybody?"`,
		`<font color="#ffff00">&lt;11:59 PM OOC&gt; dArKwInG666 says, "..."</font>`,
		`&lt;Travis thinks aloud: I shouldn't have done that.&gt;`,
		`<font color="#88aaff">&lt;Tobias thinks aloud: where did the cat go&gt;</font>`,
	}
	for _, raw := range drops {
		parsed := protocol.ParseHTMLWithIndent(raw, 0)
		if _, _, hit := f.Match(parsed.Text); !hit {
			t.Errorf("expected hit for parsed text %q (raw input %q)", parsed.Text, raw)
		}
	}

	keeps := []string{
		`&lt;8:14 pm OOC&gt; Marcus says, "anyone selling armor?"`, // unlisted account
		`&lt;Andrea thinks aloud: hmm.&gt;`,                        // unlisted character
		`Travis arrives from the south.`,                           // narrative, no channel
		`You hear xXSephirothXx muttering nearby.`,                 // not OOC channel
	}
	for _, raw := range keeps {
		parsed := protocol.ParseHTMLWithIndent(raw, 0)
		if _, _, hit := f.Match(parsed.Text); hit {
			t.Errorf("expected miss for parsed text %q (raw input %q)", parsed.Text, raw)
		}
	}
}
