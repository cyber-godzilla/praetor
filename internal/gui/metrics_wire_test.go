package gui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// A metric session that tracked nothing has nil Entries. The wire form must
// still marshal `entries` as [] (not null): the Svelte MetricsPanel reads
// `current.entries.length`, which throws on a null value and freezes the view.
func TestToMetricSession_EntriesNeverNil(t *testing.T) {
	got := toMetricSession(types.MetricSnapshot{Mode: "macro"})

	if got.Entries == nil {
		t.Fatal("toMetricSession Entries = nil, want non-nil so JSON emits []")
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"entries":null`) {
		t.Errorf("wire JSON has entries:null, want []: %s", b)
	}
}
