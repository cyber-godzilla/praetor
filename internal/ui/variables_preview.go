package ui

import (
	"fmt"
	"strings"

	sharedvars "github.com/cyber-godzilla/praetor/internal/variables"
)

func expansionPreview(raw string, values map[string]string, match bool) string {
	if !strings.Contains(raw, "${") && !strings.Contains(raw, `\${`) {
		return ""
	}
	expanded, err := sharedvars.Expand(raw, values)
	if err != nil {
		return "Unresolved: " + err.Error()
	}
	if match && expanded == "" {
		return "Resolves empty — matches nothing"
	}
	return fmt.Sprintf("Resolves to: %q", expanded)
}

func clonePreviewVariables(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for name, value := range values {
		cloned[name] = value
	}
	return cloned
}
