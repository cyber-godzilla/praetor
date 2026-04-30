package calc

import "testing"

// RankTierBonus values verified against the wiki calculator
// (rank-bonus-calculator iframe JS — see plan for source).
func TestRankTierBonus(t *testing.T) {
	cases := []struct {
		rank int
		want float64
	}{
		{0, 0},
		{1, 3},     // tier 1: 3 RB/rank for ranks 1..10
		{10, 30},   // 10 ranks × 3
		{11, 32},   // tier 1 (10×3=30) + tier 2 (1×2=2)
		{30, 70},   // tiers 1+2: 30 + 40
		{50, 90},   // tier 1+2+3: 30 + 40 + 20
		{100, 115}, // 30+40+20+25
		{150, 127.5},
		{200, 133.75},
		{500, 154},
		{1000, 166.5},
		{1150, 168}, // canonical screenshot input
		{2000, 176.5},
	}
	for _, tc := range cases {
		got := RankTierBonus(tc.rank)
		if got != tc.want {
			t.Errorf("RankTierBonus(%d) = %v, want %v", tc.rank, got, tc.want)
		}
	}
}
