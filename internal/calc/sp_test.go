package calc

import "testing"

// Base sp_per_rank for slot 1 across difficulties (transcribed from
// the wiki's calc_SP_Required JS): easy=5, average=7, difficult=9,
// impossible=11. Rank 0 -> rank 1 includes sp_first_rank: easy=10,
// avg=15, diff=17, impos=19. Selftrain/Selftaught/Healing all off.
func TestTrainSPCost_Slot1FromZero(t *testing.T) {
	cases := []struct {
		diff Difficulty
		want int
	}{
		{DifficultyEasy, 10},       // 1 × 5 + (10 - 5) = 10
		{DifficultyAverage, 15},    // 1 × 7 + (15 - 7) = 15
		{DifficultyDifficult, 17},  // 1 × 9 + (17 - 9) = 17
		{DifficultyImpossible, 19}, // 1 × 11 + (19 - 11) = 19
	}
	for _, tc := range cases {
		got := TrainSPCost(0, 1, 1, tc.diff, false, false, false)
		if got != tc.want {
			t.Errorf("TrainSPCost(0->1, slot=1, %v) = %d, want %d", tc.diff, got, tc.want)
		}
	}
}

// Above 1150 ranks, an additional non-selftrain layer kicks in. Verify
// for slot 1 easy training 1150 -> 2000:
//
//	base layer: (2000 - 1150) × 5 = 4250
//	1150-2000 layer (selftrain off, selftaught off): 850 × 5 × 1 = 4250
//	total = 8500
func TestTrainSPCost_Slot1Easy_1150To2000(t *testing.T) {
	got := TrainSPCost(1150, 2000, 1, DifficultyEasy, false, false, false)
	if got != 8500 {
		t.Errorf("TrainSPCost(1150->2000, slot=1, Easy) = %d, want 8500", got)
	}
}

// Self-Trained doubles the base layer. Self-Taught reduces that
// penalty to 1.5x. With both off the multiplier is 1.0.
func TestTrainSPCost_SelfTrainedSelfTaught_Slot1Easy(t *testing.T) {
	cases := []struct {
		selfTrained, selfTaught bool
		want                    int
	}{
		// 1 × 5 + (10 - 5) = 10 base
		{false, false, 10}, // 10 × 1.0
		{true, false, 20},  // 10 × 2.0
		{true, true, 15},   // 10 × 1.5
		{false, true, 10},  // self-taught alone has no base effect
	}
	for _, tc := range cases {
		got := TrainSPCost(0, 1, 1, DifficultyEasy, tc.selfTrained, tc.selfTaught, false)
		if got != tc.want {
			t.Errorf("TrainSPCost(0->1, st=%v, sl=%v) = %d, want %d",
				tc.selfTrained, tc.selfTaught, got, tc.want)
		}
	}
}

// Slot multiplier verification: every increment in slot adds 1 to
// sp_per_rank. Slot 5 easy (per-rank) = 9, slot 10 easy = 14.
func TestTrainSPCost_SlotProgression(t *testing.T) {
	// Train rank 1 -> 2 at slot 5 easy: sp_per_rank = 9, no first-rank.
	if got := TrainSPCost(1, 2, 5, DifficultyEasy, false, false, false); got != 9 {
		t.Errorf("slot 5 easy 1->2 = %d, want 9", got)
	}
	// Train rank 1 -> 2 at slot 10 easy: sp_per_rank = 14.
	if got := TrainSPCost(1, 2, 10, DifficultyEasy, false, false, false); got != 14 {
		t.Errorf("slot 10 easy 1->2 = %d, want 14", got)
	}
}

// Decrement / equal ranks return zero (you can't untrain).
func TestTrainSPCost_NoCostWhenNotIncreasing(t *testing.T) {
	if got := TrainSPCost(500, 500, 1, DifficultyEasy, false, false, false); got != 0 {
		t.Errorf("equal ranks should cost 0, got %d", got)
	}
	if got := TrainSPCost(500, 100, 1, DifficultyEasy, false, false, false); got != 0 {
		t.Errorf("decrement should cost 0, got %d", got)
	}
}

// DifficultyBasic costs the same as DifficultyEasy. The wiki training
// model has basics following the easy-difficulty rate (per-rank +
// double first-rank from zero). Tailoring/leatherworking are the
// game-side exceptions where basics is free; that's not a calculator
// concern.
func TestTrainSPCost_DifficultyBasicMatchesEasy(t *testing.T) {
	cases := []struct {
		cur, des, slot int
	}{
		{0, 1, 1},
		{0, 5, 1},
		{100, 200, 1},
		{1100, 1200, 1},
		{0, 50, 5},
	}
	for _, tc := range cases {
		basic := TrainSPCost(tc.cur, tc.des, tc.slot, DifficultyBasic, false, false, false)
		easy := TrainSPCost(tc.cur, tc.des, tc.slot, DifficultyEasy, false, false, false)
		if basic != easy {
			t.Errorf("Basic %d->%d slot=%d = %d, expected match Easy = %d",
				tc.cur, tc.des, tc.slot, basic, easy)
		}
	}
}

// Healing skills add a flat +5 SP per rank trained on top of the
// base cost. Verify across difficulties and direction-of-travel.
func TestTrainSPCost_HealingAddsFiveSPPerRank(t *testing.T) {
	cases := []struct {
		cur, des, slot int
		diff           Difficulty
		extra          int // expected delta vs healing=false
	}{
		{0, 1, 1, DifficultyEasy, 5},     // 1 rank
		{1, 11, 1, DifficultyEasy, 50},   // 10 ranks × 5
		{0, 10, 1, DifficultyAverage, 50},
		{100, 200, 5, DifficultyDifficult, 500},
	}
	for _, tc := range cases {
		off := TrainSPCost(tc.cur, tc.des, tc.slot, tc.diff, false, false, false)
		on := TrainSPCost(tc.cur, tc.des, tc.slot, tc.diff, false, false, true)
		if got := on - off; got != tc.extra {
			t.Errorf("healing extra for %d->%d slot=%d %v = %d, want %d",
				tc.cur, tc.des, tc.slot, tc.diff, got, tc.extra)
		}
	}
}

// Healing has no effect when no ranks are being trained.
func TestTrainSPCost_HealingNoOpWhenNotIncreasing(t *testing.T) {
	if got := TrainSPCost(500, 500, 1, DifficultyEasy, false, false, true); got != 0 {
		t.Errorf("healing equal ranks should cost 0, got %d", got)
	}
}
