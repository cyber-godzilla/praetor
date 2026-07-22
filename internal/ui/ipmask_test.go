package ui

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestMaskIPs_BoundariesAndFalsePositives(t *testing.T) {
	ipMaskCache = make(map[string]string) // deterministic start

	cases := []struct {
		in         string
		shouldMask bool
	}{
		{"1.2.3.4", true},
		{"visit 8.8.8.8 now", true},
		{"(1.2.3.4)", true},  // punctuation boundary still masked
		{"1.2.3.4.5", false}, // 5-octet: not an IPv4, leave it entirely
		{"999.1.1.1", false}, // octet > 255
		{"1.2.3", false},     // only 3 octets
		{"version 1.2", false},
	}
	for _, tc := range cases {
		out := maskIPs([]types.StyledSegment{{Text: tc.in}})[0].Text
		masked := out != tc.in
		if masked != tc.shouldMask {
			t.Errorf("maskIPs(%q) = %q (masked=%v), want masked=%v", tc.in, out, masked, tc.shouldMask)
		}
	}

	// Session-consistency: the same real IP masks to the same fake.
	a := maskIPs([]types.StyledSegment{{Text: "5.6.7.8"}})[0].Text
	b := maskIPs([]types.StyledSegment{{Text: "5.6.7.8"}})[0].Text
	if a != b {
		t.Errorf("inconsistent mask for the same IP: %q vs %q", a, b)
	}
}
