package gui

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/cyber-godzilla/praetor/internal/update"
)

// updateEndpoint is the release feed the update check queries; a package var
// so tests can point it at a local server.
var updateEndpoint = update.DefaultEndpoint

// CheckForUpdate reports whether a newer release exists. The frontend calls it
// once shortly after startup and shows a toast when Available is true.
// Disabled via updates.check. Failures are logged and reported as "no update"
// so a flaky network can never surface an error at boot.
func (a *GuiApp) CheckForUpdate() update.Info {
	if !a.cfg().Updates.Check {
		return update.Info{Current: a.deps.Version}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := update.Check(ctx, &http.Client{Timeout: 10 * time.Second}, updateEndpoint, a.deps.Version)
	if err != nil {
		log.Printf("update check failed: %v", err)
	}
	return info
}
