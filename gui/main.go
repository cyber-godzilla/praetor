// Command praetor-gui is the Wails desktop frontend for praetor. It is a thin
// shell: it bootstraps the shared core (internal/gui.Bootstrap), constructs the
// GuiApp facade, and wires the Wails runtime's event emitter to it. All game
// logic lives in the core packages; nothing game-related is implemented here.
//
// This package imports the Wails runtime, which links against WebView2
// (Windows) / WebKit (macOS/Linux) via cgo. It therefore only builds on a
// machine with the platform webview toolchain installed — see gui/README.md.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cyber-godzilla/praetor/internal/gui"
	versioninfo "github.com/cyber-godzilla/praetor/internal/version"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// version is set at build time via ldflags (-X main.version=...). A runtime
// initializer would clobber the linker value, so it defaults to "" and falls
// back to the shared embedded version (internal/version) in main().
var version = ""

//go:embed all:frontend/dist
var assets embed.FS

// wailsEmitter implements gui.Emitter by forwarding to the Wails runtime.
// The context is captured at startup; emits before startup are dropped.
type wailsEmitter struct {
	ctx context.Context
}

func (e *wailsEmitter) Emit(event string, data any) {
	if e.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(e.ctx, event, data)
}

func main() {
	debug := flag.Bool("debug", false, "Enable the debug panel and debug logging")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if version == "" {
		version = versioninfo.Version
	}

	if *showVersion {
		fmt.Printf("praetor-gui %s\n", version)
		os.Exit(0)
	}

	deps, err := gui.Bootstrap(version, *debug)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer deps.Close()

	emitter := &wailsEmitter{}
	app := gui.NewGuiApp(deps, emitter)

	err = wails.Run(&options.App{
		Title:  "Praetor — The Eternal City",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 20, G: 20, B: 24, A: 255},
		OnStartup: func(ctx context.Context) {
			// Capture the runtime context so the emitter can push events.
			emitter.ctx = ctx
		},
		Bind: []any{
			app,
		},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}
