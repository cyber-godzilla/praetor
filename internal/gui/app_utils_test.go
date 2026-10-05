package gui

import "testing"

func TestCalcTrainingCostsBuildsAllTwentySlots(t *testing.T) {
	rows := (*GuiApp)(nil).CalcTrainingCosts(0, 0, 1, 1, false, false, false)
	if len(rows) != 20 {
		t.Fatalf("len(rows) = %d, want 20", len(rows))
	}
	first := rows[0]
	if first.Slot != 1 || first.Basic != 10 || first.Easy != 10 || first.Average != 15 || first.Difficult != 17 || first.Impossible != 19 {
		t.Fatalf("slot 1 = %+v, want canonical first-rank costs", first)
	}
	if rows[19].Slot != 20 {
		t.Fatalf("last slot = %d, want 20", rows[19].Slot)
	}
}

func TestCalcTrainingCostsUsesBasicsForBasicAndSubskillForOtherColumns(t *testing.T) {
	rows := (*GuiApp)(nil).CalcTrainingCosts(0, 100, 1, 100, false, false, false)
	if rows[0].Basic == 0 {
		t.Fatal("basic column should use the increasing basics range")
	}
	if rows[0].Easy != 0 || rows[0].Average != 0 || rows[0].Difficult != 0 || rows[0].Impossible != 0 {
		t.Fatalf("non-basic columns should use unchanged subskill range: %+v", rows[0])
	}
}
