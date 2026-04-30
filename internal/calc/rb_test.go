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

// The canonical screenshot from the wiki: basics=1150, subskill=500,
// Defensive mode. Every cell of the 5-posture × 5-difficulty table
// must match what the wiki calculator displays.
func TestRankBonus_DefensiveScreenshotCase(t *testing.T) {
	cases := []struct {
		posture    Posture
		difficulty Difficulty
		want       float64
	}{
		// Basic column = floor(basicsRB) × stanceMod (no subskill contribution).
		{PostureDefensive, DifficultyBasic, 168}, // 168 × 1.0
		{PostureWary, DifficultyBasic, 126},      // 168 × 0.75
		{PostureNormal, DifficultyBasic, 84},     // 168 × 0.5
		{PostureAggressive, DifficultyBasic, 42}, // 168 × 0.25
		{PostureBerserk, DifficultyBasic, 0},     // 168 × 0
		// Easy column = (floor(basicsRB)×0.75 + subRB) × stanceMod = (126+154) × stanceMod = 280 × stanceMod
		{PostureDefensive, DifficultyEasy, 280},
		{PostureWary, DifficultyEasy, 210},
		{PostureNormal, DifficultyEasy, 140},
		{PostureAggressive, DifficultyEasy, 70},
		{PostureBerserk, DifficultyEasy, 0},
		// Average column = (84 + 154) × stanceMod = 238 × stanceMod
		{PostureDefensive, DifficultyAverage, 238},
		{PostureWary, DifficultyAverage, 178.5},
		{PostureNormal, DifficultyAverage, 119},
		{PostureAggressive, DifficultyAverage, 59.5},
		{PostureBerserk, DifficultyAverage, 0},
		// Difficult column = (42 + 154) × stanceMod = 196 × stanceMod
		{PostureDefensive, DifficultyDifficult, 196},
		{PostureWary, DifficultyDifficult, 147},
		{PostureNormal, DifficultyDifficult, 98},
		{PostureAggressive, DifficultyDifficult, 49},
		{PostureBerserk, DifficultyDifficult, 0},
		// Impossible column = (16.8 + 154) × stanceMod = 170.8 × stanceMod
		{PostureDefensive, DifficultyImpossible, 170.8},
		{PostureWary, DifficultyImpossible, 128.1},
		{PostureNormal, DifficultyImpossible, 85.4},
		{PostureAggressive, DifficultyImpossible, 42.7},
		{PostureBerserk, DifficultyImpossible, 0},
	}
	for _, tc := range cases {
		got := RankBonus(ModeDefensive, 1150, 500, tc.posture, tc.difficulty)
		if !approxEqual(got, tc.want) {
			t.Errorf("RankBonus(Def, 1150, 500, %v, %v) = %v, want %v",
				tc.posture, tc.difficulty, got, tc.want)
		}
	}
}

// Noncombat mode = same RB as Defensive-defensive / Offensive-berserk
// (per spec). Single row of difficulty values, no posture variation.
func TestRankBonus_NoncombatScreenshotCase(t *testing.T) {
	cases := []struct {
		difficulty Difficulty
		want       float64
	}{
		{DifficultyBasic, 168},
		{DifficultyEasy, 280},
		{DifficultyAverage, 238},
		{DifficultyDifficult, 196},
		{DifficultyImpossible, 170.8},
	}
	for _, tc := range cases {
		// Posture argument is ignored for ModeNoncombat; pass zero.
		got := RankBonus(ModeNoncombat, 1150, 500, 0, tc.difficulty)
		if !approxEqual(got, tc.want) {
			t.Errorf("RankBonus(Noncombat, 1150, 500, _, %v) = %v, want %v",
				tc.difficulty, got, tc.want)
		}
	}
}

// Offensive mode: stance modifiers run the opposite direction —
// Berserk gets the full 1.0 multiplier, Defensive gets 0.
func TestRankBonus_OffensiveScreenshotCase(t *testing.T) {
	// Berserk-Easy = (floor(168) × 0.75 + 154) × 1.0 = 280 (full benefit)
	if got := RankBonus(ModeOffensive, 1150, 500, PostureBerserk, DifficultyEasy); !approxEqual(got, 280) {
		t.Errorf("Offensive Berserk Easy = %v, want 280", got)
	}
	// Defensive-Easy = 280 × 0.0 = 0 in offensive mode
	if got := RankBonus(ModeOffensive, 1150, 500, PostureDefensive, DifficultyEasy); !approxEqual(got, 0) {
		t.Errorf("Offensive Defensive Easy = %v, want 0", got)
	}
	// Normal-Easy = 280 × 0.5 = 140 in either mode
	if got := RankBonus(ModeOffensive, 1150, 500, PostureNormal, DifficultyEasy); !approxEqual(got, 140) {
		t.Errorf("Offensive Normal Easy = %v, want 140", got)
	}
}

// Floating-point comparison helper. RB values are decimal (e.g. 170.8,
// 128.1) so a tolerance is needed.
func approxEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.001
}
