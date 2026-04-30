package calc

import "math"

// spBase encodes the (sp_per_rank, sp_first_rank_constant) values
// from the wiki's calc_SP_Required for each difficulty. The
// per-rank value is `perRank + (slot - 1)`; the first-rank
// adjustment is `firstAdj + (slot - 1)*3 - perRank`.
type spBase struct {
	perRank  int
	firstAdj int
}

var spBases = map[Difficulty]spBase{
	DifficultyEasy:       {perRank: 5, firstAdj: 10},
	DifficultyAverage:    {perRank: 7, firstAdj: 15},
	DifficultyDifficult:  {perRank: 9, firstAdj: 17},
	DifficultyImpossible: {perRank: 11, firstAdj: 19},
}

// TrainSPCost returns the total SP needed to train a single skill
// from curRank to desRank at the given slot (1..20) and difficulty.
// Returns 0 when desRank <= curRank. selfTrained applies the
// no-trainer penalty multiplier (2x base layer); selfTaught reduces
// that penalty (1.5x) and also reduces the 1150-2000 layer multiplier
// (0.5x), per the wiki JS. DifficultyBasic is invalid here (it's not
// a trainable difficulty) and returns 0.
func TrainSPCost(curRank, desRank, slot int, diff Difficulty, selfTrained, selfTaught bool) int {
	if desRank <= curRank {
		return 0
	}
	base, ok := spBases[diff]
	if !ok {
		return 0
	}
	if slot < 1 {
		slot = 1
	}
	spPerRank := float64(base.perRank + (slot - 1))
	spFirstRank := float64(base.firstAdj+(slot-1)*3) - spPerRank

	cur := float64(curRank)
	des := float64(desRank)

	// Base layer: (des - cur) * spPerRank, plus first-rank adjustment
	// when starting from 0. Multiplied by selftrain/selftaught penalty.
	baseLayer := (des-cur)*spPerRank + condFloat(curRank == 0, spFirstRank, 0)
	stMult := 1.0
	if selfTrained {
		if selfTaught {
			stMult = 1.5
		} else {
			stMult = 2.0
		}
	}
	total := baseLayer * stMult

	// 1150-2000 layer (only applies when NOT selftrained per wiki JS).
	if desRank > 1150 && curRank <= 2000 && !selfTrained {
		layer := (math.Min(des, 2000) - math.Max(cur, 1150)) * spPerRank
		mult := 1.0
		if selfTaught {
			mult = 0.5
		}
		total += layer * mult
	}

	// 2000-3000 layer.
	if desRank > 2000 && curRank <= 3000 {
		layer := (math.Min(des, 3000) - math.Max(cur, 2000)) * spPerRank
		mult := 3.0
		if selfTrained {
			mult = 2.0
		} else if selfTaught {
			mult = 2.5
		}
		total += layer * mult
	}

	// 3000-4000.
	if desRank > 3000 && curRank <= 4000 {
		layer := (math.Min(des, 4000) - math.Max(cur, 3000)) * spPerRank
		mult := 5.0
		if selfTrained {
			mult = 4.0
		} else if selfTaught {
			mult = 4.5
		}
		total += layer * mult
	}

	// 4000-5000.
	if desRank > 4000 && curRank <= 5000 {
		layer := (math.Min(des, 5000) - math.Max(cur, 4000)) * spPerRank
		mult := 7.0
		if selfTrained {
			mult = 6.0
		} else if selfTaught {
			mult = 6.5
		}
		total += layer * mult
	}

	// 5000-7000.
	if desRank > 5000 && curRank <= 7000 {
		layer := (math.Min(des, 7000) - math.Max(cur, 5000)) * spPerRank
		mult := 9.0
		if selfTrained {
			mult = 8.0
		} else if selfTaught {
			mult = 8.5
		}
		total += layer * mult
	}

	// 7000+.
	if desRank > 7000 {
		layer := (des - math.Max(cur, 7000)) * spPerRank
		mult := 11.0
		if selfTrained {
			mult = 10.0
		} else if selfTaught {
			mult = 10.5
		}
		total += layer * mult
	}

	return int(math.Round(total))
}

func condFloat(cond bool, t, f float64) float64 {
	if cond {
		return t
	}
	return f
}
