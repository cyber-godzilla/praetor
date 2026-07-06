package gui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cyber-godzilla/praetor/internal/calc"
	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/wiki"
)

// ---------------------------------------------------------------------------
// Kudos
// ---------------------------------------------------------------------------

// GetKudos returns the current kudos configuration (favorites + queue).
func (a *GuiApp) GetKudos() config.KudosConfig { return a.cfg().Kudos }

// SetKudos replaces the kudos configuration.
func (a *GuiApp) SetKudos(k config.KudosConfig) error {
	a.cfg().Kudos = k
	return a.save()
}

// AddKudosFavorite adds a favorite if not present. Returns true if added.
func (a *GuiApp) AddKudosFavorite(name string) (bool, error) {
	if a.cfg().Kudos.HasFavorite(name) {
		return false, nil
	}
	a.cfg().Kudos.AddFavorite(name)
	return true, a.save()
}

// AddKudosQueue queues a kudos for the named character.
func (a *GuiApp) AddKudosQueue(name, message string) error {
	a.cfg().Kudos.AddQueueEntry(name, message)
	return a.save()
}

// ---------------------------------------------------------------------------
// Persistent Lua state
// ---------------------------------------------------------------------------

// PersistentKeyInfo describes one persisted state key for the data manager.
type PersistentKeyInfo struct {
	Key          string `json:"key"`
	ValueSummary string `json:"valueSummary"`
}

// GetPersistentData returns a summary of all persisted Lua state keys.
func (a *GuiApp) GetPersistentData() []PersistentKeyInfo {
	state := a.client().Engine.State()
	keys := state.PersistentKeys()
	snap := state.PersistentSnapshot()
	infos := make([]PersistentKeyInfo, 0, len(keys))
	for _, key := range keys {
		summary := ""
		if val, ok := snap[key]; ok {
			summary = describePersistentValue(val)
		}
		infos = append(infos, PersistentKeyInfo{Key: key, ValueSummary: summary})
	}
	return infos
}

// ExportPersistentData writes the selected keys to a timestamped JSON file in
// the config exports dir and returns the written path.
func (a *GuiApp) ExportPersistentData(keys []string) (string, error) {
	snap := a.client().Engine.State().PersistentSnapshot()
	exportData := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		if val, ok := snap[key]; ok {
			exportData[key] = val
		}
	}
	exportDir := filepath.Join(a.deps.ConfigDir, "exports")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("persistent_%s.json", time.Now().Format("2006-01-02_150405"))
	exportPath := filepath.Join(exportDir, filename)
	out, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(exportPath, out, 0o644); err != nil {
		return "", err
	}
	return exportPath, nil
}

// ClearPersistentData removes the selected persisted keys and flushes to disk.
func (a *GuiApp) ClearPersistentData(keys []string) error {
	state := a.client().Engine.State()
	for _, key := range keys {
		state.ClearPersistentKey(key)
	}
	if store := a.client().Engine.PersistentStore(); store != nil {
		store.Flush()
	}
	return nil
}

func describePersistentValue(val interface{}) string {
	switch v := val.(type) {
	case map[string]interface{}:
		return fmt.Sprintf("%d entries", len(v))
	case float64:
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", int(v))
		}
		return fmt.Sprintf("%g", v)
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Wiki / maps bookmarks
// ---------------------------------------------------------------------------

// GetWikiSections returns the wiki bookmark sections.
func (a *GuiApp) GetWikiSections() []wiki.Section { return wiki.Sections() }

// GetMapSections returns the map bookmark sections.
func (a *GuiApp) GetMapSections() []wiki.Section { return wiki.MapSections() }

// OpenURL opens an arbitrary URL in the system browser.
func (a *GuiApp) OpenURL(url string) { go client.OpenBrowser(url) }

// OpenWikiSlug opens a wiki bookmark by slug.
func (a *GuiApp) OpenWikiSlug(slug string) { go client.OpenBrowser(wiki.URL(slug)) }

// ---------------------------------------------------------------------------
// Rank-bonus / training-cost calculator (primitives from internal/calc)
// ---------------------------------------------------------------------------

// RBCell is a single rank-bonus value for a posture/difficulty combination.
type RBCell struct {
	Posture    int     `json:"posture"`
	Difficulty int     `json:"difficulty"`
	Bonus      float64 `json:"bonus"`
}

// RBResult is the full posture×difficulty grid for a mode plus the basics-only
// tier bonus, letting the frontend render the calculator table.
type RBResult struct {
	Mode       int      `json:"mode"`
	Basics     int      `json:"basics"`
	Subskill   int      `json:"subskill"`
	BasicsRB   float64  `json:"basicsRB"`
	SubskillRB float64  `json:"subskillRB"`
	Cells      []RBCell `json:"cells"`
}

// CalcRankBonus computes the rank-bonus grid for a mode and basics/subskill
// ranks across all five postures and five difficulties.
func (a *GuiApp) CalcRankBonus(mode, basics, subskill int) RBResult {
	res := RBResult{
		Mode:       mode,
		Basics:     basics,
		Subskill:   subskill,
		BasicsRB:   calc.RankTierBonus(basics),
		SubskillRB: calc.RankTierBonus(subskill),
	}
	for p := calc.PostureBerserk; p <= calc.PostureDefensive; p++ {
		for d := calc.DifficultyBasic; d <= calc.DifficultyImpossible; d++ {
			res.Cells = append(res.Cells, RBCell{
				Posture:    int(p),
				Difficulty: int(d),
				Bonus:      calc.RankBonus(calc.Mode(mode), basics, subskill, p, d),
			})
		}
	}
	return res
}

// CalcTrainCost returns the skill-point cost to train from curRank to desRank
// in a given slot and difficulty, with the standard modifiers.
func (a *GuiApp) CalcTrainCost(curRank, desRank, slot, difficulty int, selfTrained, selfTaught, healing bool) int {
	return calc.TrainSPCost(curRank, desRank, slot, calc.Difficulty(difficulty), selfTrained, selfTaught, healing)
}
