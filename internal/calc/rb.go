// Package calc implements the Eternal City rank-bonus and training-
// cost formulas. All values are transcribed from the wiki's
// rank-bonus-calculator and training-cost-calculator JS sources.
package calc

import "math"

// Mode selects which calculator the rank-bonus table represents.
// Mirrors the three buttons in the wiki calculator's menu bar
// (sword=offensive, shield=defensive, tree=noncombat).
type Mode int

const (
	ModeDefensive Mode = iota
	ModeOffensive
	ModeNoncombat
)

// Posture is one of the five combat stances. The iota order matches
// the offensive ladder (Berserk=0 → Defensive=4 mapping to multipliers
// 1.0 → 0.0); defensive mode walks the same ladder reversed. UI code
// that renders the table is responsible for picking the right row
// order per mode (see renderRBTable in internal/ui/rbcalc.go).
type Posture int

const (
	PostureBerserk Posture = iota
	PostureAggressive
	PostureNormal
	PostureWary
	PostureDefensive
)

// Difficulty controls how much of the basics rank bonus contributes
// to the subskill cell. The "Basic" column doesn't apply this
// modifier — it shows just the basics RB instead.
type Difficulty int

const (
	DifficultyBasic Difficulty = iota
	DifficultyEasy
	DifficultyAverage
	DifficultyDifficult
	DifficultyImpossible
)

// difficultyMod returns the basics-contribution multiplier for the
// non-Basic columns. Transcribed from getDifficultyModifier in the
// wiki JS. DifficultyBasic returns 0 (basics don't fold into Basic).
func difficultyMod(d Difficulty) float64 {
	switch d {
	case DifficultyEasy:
		return 0.75
	case DifficultyAverage:
		return 0.5
	case DifficultyDifficult:
		return 0.25
	case DifficultyImpossible:
		return 0.1
	}
	return 0
}

// Stance ladders, indexed by Posture (Berserk=0..Defensive=4).
// Offensive: most aggressive stance gets the full multiplier.
// Defensive: most defensive stance does. The two are reverses of
// each other.
var (
	offensiveLadder = [...]float64{1.0, 0.75, 0.5, 0.25, 0.0}
	defensiveLadder = [...]float64{0.0, 0.25, 0.5, 0.75, 1.0}
)

// stanceMod returns the rank-bonus multiplier for a (mode, posture)
// pair. Noncombat ignores stance entirely (always 1.0).
func stanceMod(m Mode, p Posture) float64 {
	switch m {
	case ModeOffensive:
		return offensiveLadder[p]
	case ModeDefensive:
		return defensiveLadder[p]
	}
	return 1.0
}

// RankBonus computes the rank-bonus value for a (mode, basics,
// subskill, posture, difficulty) tuple. Returns a float64 because
// some cells (e.g. Impossible-Wary) are non-integer.
//
// Formula (transcribed from calcRankBonus in the wiki JS):
//
//	RB = (floor(basicsTierRB) × difficultyMod + subskillTierRB) × stanceMod
//
// The "Basic" difficulty column is special-cased — it doesn't fold
// the subskill RB into the result, just the basics RB at this stance.
func RankBonus(mode Mode, basics, subskill int, posture Posture, difficulty Difficulty) float64 {
	basicsRB := math.Floor(RankTierBonus(basics))
	subRB := RankTierBonus(subskill)
	sm := stanceMod(mode, posture)

	if difficulty == DifficultyBasic {
		return basicsRB * sm
	}
	return (basicsRB*difficultyMod(difficulty) + subRB) * sm
}

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
