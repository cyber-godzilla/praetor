# praetor-gui — Wails desktop frontend

A cross-platform desktop GUI for praetor, built with [Wails v2](https://wails.io)
(Go backend + web frontend in the OS webview). It reuses the entire praetor
core (`internal/client`, `engine`, `session`, `protocol`, `minimap`, `compass`,
`config`) untouched — this module only adds a thin Wails shell (`main.go`) and a
Svelte frontend.

## Architecture

```
frontend/ (Svelte + TS + Vite)   ← UI: output, tabs, input, status, minimap, settings
     │  window.go.gui.GuiApp.*   (method calls)
     │  runtime EventsOn         (event stream)
main.go (this module)            ← Wails wiring + event emitter (webview-linked)
     │
internal/gui.GuiApp  (parent module, wails-free, unit-tested)
     │
internal/{client,engine,session,protocol,minimap,compass,config}  ← shared core
```

`internal/gui` holds all the facade logic and is **Wails-free**, so it is unit
tested with the normal `go test ./...` in the parent module. This module
(`gui/`) is a **nested Go module** so the parent repo's `make check` never tries
to compile the webview-linked code.

The minimap and compass are rendered by the existing Go code to `image.RGBA`,
PNG-encoded, and sent to the frontend as base64 data URIs — no Kitty/Sixel
terminal graphics, so they work identically on every OS (this is the main reason
the GUI fixes Windows).

## Prerequisites

- **Go** 1.25+
- **Node.js** 18+ and npm
- **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Platform webview toolchain:
  - **Windows**: WebView2 runtime (preinstalled on Win10/11).
  - **macOS**: Xcode command-line tools.
  - **Linux**: GTK3 + WebKit2GTK dev packages. Modern distros (Ubuntu 24.04+,
    incl. 25.10) ship **4.1**, which requires the `webkit2_41` build tag:
    ```bash
    sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
    ```
    Older distros with 4.0 can omit the tag. The `Makefile` always passes
    `-tags webkit2_41` (harmless on macOS/Windows), so you never have to.

Run `wails doctor` to verify your environment.

## Develop

```bash
cd gui
make dev          # wails dev -tags webkit2_41
```

Live-reloads both the Go backend and the Svelte frontend.

## Build a distributable

```bash
cd gui
make build              # -> build/bin/praetor(.exe/.app)
make installer          # Windows: build + NSIS installer
make build VERSION=v0.1.0   # stamp a version

# Or invoke wails directly (remember the tag on webkit-4.1 systems):
wails build -tags webkit2_41 -ldflags "-X main.version=v0.1.0"
```

## Working on the frontend alone

The frontend builds and type-checks without the Go toolchain or a webview:

```bash
cd gui/frontend
npm install
npm run build          # svelte-check + vite build -> dist/
npm run dev            # plain browser preview (Go calls return safe defaults)
```

When run outside Wails, `window.go`/`window.runtime` are absent; the bridge
(`src/lib/bridge.ts`) returns safe defaults so the layout still renders.

## What can be verified where

| Layer | Verifiable without webview? |
|-------|------------------------------|
| Core (`internal/*`) | ✅ `make test` in parent |
| Facade (`internal/gui`) | ✅ `go test ./internal/gui` |
| Wails glue (`gui/main.go`) | ✅ compiles with plain `go build`; full `wails build` needs the webview toolchain (WebView2 / WebKitGTK-4.1) |
| Frontend | ✅ `npm run build` |
| Full `wails build` | ✅ on any machine with the platform webview + (on Linux) `-tags webkit2_41` |
