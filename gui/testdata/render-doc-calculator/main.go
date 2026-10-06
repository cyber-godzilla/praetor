// Command render-doc-calculator creates the rank-bonus and training-cost
// fixture used by the documentation screenshot test. Values come directly
// from the production calculator package so documentation never displays
// illustrative or hand-maintained numbers.
package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"github.com/cyber-godzilla/praetor/internal/calc"
)

type rbCell struct {
	Posture    int     `json:"posture"`
	Difficulty int     `json:"difficulty"`
	Bonus      float64 `json:"bonus"`
}

type rbResult struct {
	Mode       int      `json:"mode"`
	Basics     int      `json:"basics"`
	Subskill   int      `json:"subskill"`
	BasicsRB   float64  `json:"basicsRB"`
	SubskillRB float64  `json:"subskillRB"`
	Cells      []rbCell `json:"cells"`
}

type trainingCostRow struct {
	Slot       int `json:"slot"`
	Basic      int `json:"basic"`
	Easy       int `json:"easy"`
	Average    int `json:"average"`
	Difficult  int `json:"difficult"`
	Impossible int `json:"impossible"`
}

type fixture struct {
	Current rbResult          `json:"current"`
	Target  rbResult          `json:"target"`
	Costs   []trainingCostRow `json:"costs"`
}

func main() {
	output := flag.String("output", "gui/frontend/artifacts/ui-screenshots/rbcalc.json", "output JSON file")
	flag.Parse()

	data := fixture{
		Current: rankBonusResult(calc.ModeDefensive, 1150, 500),
		Target:  rankBonusResult(calc.ModeDefensive, 1150, 1150),
		Costs:   trainingCosts(1150, 500, 1150, 1150),
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*output, append(encoded, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func rankBonusResult(mode calc.Mode, basics, subskill int) rbResult {
	result := rbResult{
		Mode:       int(mode),
		Basics:     basics,
		Subskill:   subskill,
		BasicsRB:   calc.RankTierBonus(basics),
		SubskillRB: calc.RankTierBonus(subskill),
	}
	for posture := calc.PostureBerserk; posture <= calc.PostureDefensive; posture++ {
		for difficulty := calc.DifficultyBasic; difficulty <= calc.DifficultyImpossible; difficulty++ {
			result.Cells = append(result.Cells, rbCell{
				Posture:    int(posture),
				Difficulty: int(difficulty),
				Bonus:      calc.RankBonus(mode, basics, subskill, posture, difficulty),
			})
		}
	}
	return result
}

func trainingCosts(curBasics, curSub, targetBasics, targetSub int) []trainingCostRow {
	rows := make([]trainingCostRow, 0, 20)
	for slot := 1; slot <= 20; slot++ {
		rows = append(rows, trainingCostRow{
			Slot:       slot,
			Basic:      calc.TrainSPCost(curBasics, targetBasics, slot, calc.DifficultyBasic, false, false, false),
			Easy:       calc.TrainSPCost(curSub, targetSub, slot, calc.DifficultyEasy, false, false, false),
			Average:    calc.TrainSPCost(curSub, targetSub, slot, calc.DifficultyAverage, false, false, false),
			Difficult:  calc.TrainSPCost(curSub, targetSub, slot, calc.DifficultyDifficult, false, false, false),
			Impossible: calc.TrainSPCost(curSub, targetSub, slot, calc.DifficultyImpossible, false, false, false),
		})
	}
	return rows
}
