package ui

import (
	"strings"
	"testing"
)

func TestExpansionPreview(t *testing.T) {
	values := map[string]string{"target": "Apula", "empty": ""}
	if got := expansionPreview("${target}*", values, true); !strings.Contains(got, "Apula*") {
		t.Fatalf("resolved preview = %q", got)
	}
	if got := expansionPreview("${missing}", values, true); !strings.Contains(got, "Unresolved") {
		t.Fatalf("unresolved preview = %q", got)
	}
	if got := expansionPreview("${empty}", values, true); !strings.Contains(got, "matches nothing") {
		t.Fatalf("empty match preview = %q", got)
	}
	if got := expansionPreview("literal", values, true); got != "" {
		t.Fatalf("literal preview = %q, want empty", got)
	}
}
