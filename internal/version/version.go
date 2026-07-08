// Package version is the single source of truth for the praetor version
// string, shared by the TUI (cmd/praetor) and the GUI (gui/) so the two never
// drift. The embedded VERSION file provides the default; a build may override
// it via ldflags (-X ...version.Version=v1.2.3).
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Version is the current praetor version (VERSION file contents, trimmed).
// Overridable at link time via -ldflags "-X .../internal/version.Version=...".
var Version = strings.TrimSpace(raw)
