package calc

import "testing"

// Base sp_per_rank for slot 1 across difficulties (transcribed from
// the wiki's calc_SP_Required JS): easy=5, average=7, difficult=9,
// impossible=11. Rank 0 -> rank 1 includes sp_first_rank: easy=10,
// avg=15, diff=17, impos=19. Selftrain/Selftaught both off.
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
		got := TrainSPCost(0, 1, 1, tc.diff, false, false)
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
	got := TrainSPCost(1150, 2000, 1, DifficultyEasy, false, false)
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
		got := TrainSPCost(0, 1, 1, DifficultyEasy, tc.selfTrained, tc.selfTaught)
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
	if got := TrainSPCost(1, 2, 5, DifficultyEasy, false, false); got != 9 {
		t.Errorf("slot 5 easy 1->2 = %d, want 9", got)
	}
	// Train rank 1 -> 2 at slot 10 easy: sp_per_rank = 14.
	if got := TrainSPCost(1, 2, 10, DifficultyEasy, false, false); got != 14 {
		t.Errorf("slot 10 easy 1->2 = %d, want 14", got)
	}
}

// Decrement / equal ranks return zero (you can't untrain).
func TestTrainSPCost_NoCostWhenNotIncreasing(t *testing.T) {
	if got := TrainSPCost(500, 500, 1, DifficultyEasy, false, false); got != 0 {
		t.Errorf("equal ranks should cost 0, got %d", got)
	}
	if got := TrainSPCost(500, 100, 1, DifficultyEasy, false, false); got != 0 {
		t.Errorf("decrement should cost 0, got %d", got)
	}
}
