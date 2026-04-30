// Package calc implements the Eternal City rank-bonus and training-
// cost formulas. All values are transcribed from the wiki's
// rank-bonus-calculator and training-cost-calculator JS sources.
package calc

// rankTier describes a band of skill ranks. Within a band, each rank
// contributes `bonus` rank-bonus. The band covers ranks strictly
// greater than `endLevel` up to (and including) the next band's
// endLevel — so a slice of rankTiers must be sorted by ascending
// endLevel for RankTierBonus to walk it correctly.
type rankTier struct {
	endLevel int     // floor: ranks > endLevel fall into this tier
	bonus    float64 // RB awarded per rank in this tier
}

// rankTiers mirrors rankTierModifiers / rankTierModifierEndLevels in
// the wiki rank-bonus-calculator JS. Each entry's endLevel is the
// FLOOR for that tier (ranks > endLevel get this bonus rate); the
// CEILING is implicit — it's the next entry's endLevel. The final
// entry (1000, 0.01) covers all ranks above 1000.
//
// The slice MUST be sorted by ascending endLevel.
var rankTiers = []rankTier{
	{endLevel: 0, bonus: 3},        // ranks 1..10 at 3 RB/rank
	{endLevel: 10, bonus: 2},       // ranks 11..30 at 2 RB/rank
	{endLevel: 30, bonus: 1},       // ranks 31..50 at 1 RB/rank
	{endLevel: 50, bonus: 0.5},     // ranks 51..100 at 0.5 RB/rank
	{endLevel: 100, bonus: 0.25},   // ranks 101..150 at 0.25 RB/rank
	{endLevel: 150, bonus: 0.125},  // ranks 151..200 at 0.125 RB/rank
	{endLevel: 200, bonus: 0.0675}, // ranks 201..500 at 0.0675 RB/rank
	{endLevel: 500, bonus: 0.025},  // ranks 501..1000 at 0.025 RB/rank
	{endLevel: 1000, bonus: 0.01},  // ranks above 1000 at 0.01 RB/rank
}

// RankTierBonus returns the rank-tier bonus for a given rank value.
// Walks rankTiers from highest endLevel down: each iteration peels
// off all ranks strictly above the current entry's endLevel and
// credits them at that entry's bonus rate, then caps the remaining
// rank pool at endLevel before the next iteration consumes its
// share. Returns 0 for non-positive ranks.
func RankTierBonus(rank int) float64 {
	if rank <= 0 {
		return 0
	}
	bonus := 0.0
	r := rank
	for i := len(rankTiers) - 1; i >= 0; i-- {
		end := rankTiers[i].endLevel
		if r > end {
			ranksInTier := r - end
			bonus += float64(ranksInTier) * rankTiers[i].bonus
			r = end
		}
	}
	return bonus
}
