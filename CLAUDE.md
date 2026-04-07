# CLAUDE.md

## Project Overview

This is **praetor**, a Go terminal-based game client for The Eternal City (TEC), built with Bubbletea/Lipgloss. It replaces the browser-based Orchil client entirely, including authentication, game interaction, automation via Lua scripting, and a minimap renderer using Kitty graphics protocol.

**Repository:** `github.com/cyber-godzilla/praetor`

## Architecture

### Core Library + TUI Shell (all under `internal/`)

```
praetor/
├── cmd/praetor/main.go              # Entry point with wrapper pattern for Bubbletea
├── internal/
│   ├── client/                      # Client orchestrator: session, engine, protocol, notifications
│   ├── colorwords/                  # Color word detection and data for game text
│   ├── compass/                     # Compass rose renderer (Kitty graphics)
│   ├── config/                      # YAML config loading + saving, validation
│   ├── engine/                      # Lua VM, mode loading, pattern matching, command queue, timers, metrics, persistent state
│   ├── kitty/                       # Kitty graphics protocol encoding
│   ├── logging/                     # Structured logging with rotation
│   ├── minimap/                     # Minimap renderer (Kitty graphics)
│   ├── protocol/                    # Line buffer, SKOOT parsing, HTML parsing
│   ├── session/                     # WebSocket, HTTP auth, keyring (multi-account), reconnection
│   ├── types/                       # Shared event types
│   └── ui/                          # Bubbletea TUI components
├── Makefile                         # make test, make build, make run, make vet, make fmt, make lint, make check
└── .gitignore
```

## Development Commands

- `make test` — runs all tests (`go test ./... -count=1 -timeout=60s`)
- `make build` — builds `./praetor` binary
- `make run` — builds and launches the TUI
- `make vet` — runs `go vet ./...`
- `make fmt` — runs `gofmt -l .` (lists unformatted files)
- `make lint` — runs `staticcheck ./...` (if installed)
- `make check` — runs vet + fmt + lint + test

## Git Workflow

- All work is on `main` branch

## Authentication Flow

Two-step HTTP login + WebSocket + SKOOT handshake:

1. **GET** `https://login.eternalcitygame.com/login.php` — receives `biscuit=test` cookie
2. **POST** same URL with `submit=true&phrase=&uname=<user>&pwd=<password>` + biscuit cookie — receives 302 with `user`/`pass` cookies (pass is a numeric token, NOT the raw password)
3. **WebSocket** connects to `ws://game.eternalcitygame.com:8080/tec` with user/pass cookies
4. Client sends: `SKOTOS Praetor 0.1.0`
5. Server sends: `SECRET <token>`
6. Client sends: `USER <username>`, `SECRET <token>`, `HASH <md5(username + passCookie + token)>`
7. Credentials stored in system keyring (multi-account via JSON blob)

## SKOOT Protocol

| Channel | Purpose | Format |
|---------|---------|--------|
| 6 | Minimap rooms | Groups of 5: `x,y,size,#color,brightness` — x,y are top-left positions |
| 7 | Exits | Pairs: `n,show,ne,none,...` |
| 8 | Status bars | `Name,value` (Health, Fatigue, Encumbrance, Satiation) |
| 9 | Lighting | Single int (0-30 Orchil range; 30-100 Very Bright; 100+ Extremely Bright) |
| 10 | Minimap walls | Groups of 4: `x,y,type,accessible` (1=accessible/white, 0=blocked) |

Wall types: `hor` (horizontal), `ver` (vertical), `ne` (diagonal NE-SW), `nw` (diagonal NW-SE). Prefix `no` means blocked: `none` = blocked NE, `nonw` = blocked NW.

## Minimap Rendering (V2 — Kitty Graphics)

The minimap renders rooms and walls to a pixel image (`image.RGBA`) and displays it inline using the **Kitty graphics protocol**. This gives full-color pixel-accurate rendering matching Orchil's visual style.

**Room rendering:**
- SKOOT x,y used as direct pixel positions (top-left corner)
- Rooms drawn at full SKOOT width (no size-2 inset)
- Filled with brightness-adjusted color, then 1px black outline on top
- Rooms drawn in SKOOT order (later rooms paint over earlier ones)

**Wall/passage rendering:**
- SKOOT 10 coordinates used directly — no adjacency detection
- Direction-based line rendering with configurable offsets:
  - `hor`: horizontal line ±offset from wall position
  - `ver`: vertical line ±offset
  - `ne`/`nw`: diagonal lines ±offset
- Passable walls: white lines. Blocked walls: black lines
- Walls only rendered when 2+ rooms are visible

**Brightness formula:** `clamp(brightness, 0, 30)`, then `(brightness - 25) * 8` added to each RGB channel.

**Adaptive scaling:** Scale = min(configured, room_size_cap at 40%, nearby_bounding_box_fit). Small room boost (2x) when all rooms ≤ size 10.

**Compass:** Also rendered via Kitty graphics (6 rows). Triangular arrows for cardinals, right-angle triangles for diagonals. Green=active, dim=inactive. Center dot with up/down indicators.

**Kitty protocol integration:** Escape sequences injected after Lipgloss layout via ANSI cursor positioning. `C=1` flag prevents cursor movement. Images chunked at 4096 bytes.

## Lua Engine

- **gopher-lua** (pure Go Lua 5.1 VM)
- Script directories configurable via `config.yaml` `scripts:` field or menu
- Each directory is added to Lua `package.path` and scanned for modes
- Default: `~/.config/praetor/scripts/`
- Hot reload via Esc → Reload Scripts — clears `package.loaded` cache so lib changes take effect
- Timer support: `set_timeout(fn, ms)`, `set_interval(fn, ms)`, `clear_timer(id)` — auto-cancel on mode switch
- Pattern matching in Go (substring + wildcard→regex), Lua only called on match
- Action functions receive the matched text as first argument: `action = function(text)`

### Lua API

```lua
send(command [, delay_ms])           -- queue game command
set_mode(name [, {args}])           -- switch mode
notify(title, message)               -- desktop notification
log(message)
random_item(table)
time.now() / time.since(ms)
state.get(key) / state.set(key, val) -- per-mode state
state.persist(key)                   -- mark key for disk persistence
state.mode
status.health / status.fatigue / status.encumbrance / status.satiation
metrics.track(key, label)            -- declare a metric for current session
metrics.inc(key) / metrics.dec(key)  -- increment/decrement a metric
metrics.set(key, value) / metrics.get(key)
set_timeout(fn, ms) / set_interval(fn, ms) / clear_timer(id)
```

## Key Bindings (Game View)

| Key | Action |
|-----|--------|
| Tab | Next tab |
| Shift+Tab | Previous tab |
| Alt+1..9, Alt+0 | Jump to tab N (0 = 10th) |
| Alt+S | Toggle sidebar |
| Alt+M | Quick-cycle modes (persisted to config) |

| Esc | Open menu |
| Ctrl+C | Clear input / confirm quit |
| PgUp/PgDn | Scroll output |
| Mouse wheel | Scroll output (3 lines per tick) |
| Enter (empty) | Send blank line to server |

### Reserved Alt Keys (do NOT bind)
A, B, C, D, F, H, J, K, L, R, T, U, Y, Z, [ — VT100/readline/Ghostty conflicts

### Available Alt Keys for future bindings
E, G, N, O, P, Q, V, W, X

## Tabs

- **All** — always present, receives all game text
- **Custom tabs** — user-defined via Esc → Custom Tabs, with include/exclude pattern rules
- **Metrics** — session metrics dashboard (kills, actions, duration, history)
- **Debug** — raw SKOOT protocol data (visible only with `--debug` flag)

## String Highlighting

Configurable pattern highlighting for loot detection. Managed via Esc → Highlights menu.
- Case-insensitive substring matching on all game text
- Four color styles: red (red bg/white fg), gold (yellow bg/black fg), green (green bg/black fg), blue (blue bg/white fg)
- Per-pattern active toggle, style cycling, add/delete
- Persisted to `config.yaml` → `highlights:` section

## HTML Parsing

- `<b>` → bold, `<i>` → italic, `<font color>` → color (dark colors like #000000 skipped)
- `<font size=+1>` → bold + orange (titlebar)
- `<hr>` → horizontal rule segment (IsHR flag)
- `<ul>/<li>` → indentation (tracked across protocol lines via `htmlIndent` in client)
- `<xch_cmd>` → underline, `<xch_page clear>` → clear
- `</pre>` → section break (empty line injected before)

## Text Rendering

- Word-aware line wrapping: breaks at last whitespace before width limit
- Leading whitespace preserved (custom `padLines()` avoids lipgloss Width trimming)
- Command echo: user and engine commands shown as italic when `/echo on`
- Scrollback: configurable per-tab, scroll position tracks display rows not logical lines
- Highlight patterns applied before rendering (see String Highlighting above)

## Config

`~/.config/praetor/config.yaml` (created with defaults on first run)

Config is read on startup and saved back on certain UI actions (quick-cycle changes, highlight changes).

```yaml
server:
  host: game.eternalcitygame.com
  port: 8080
  protocol: ws
  login_url: https://login.eternalcitygame.com/login.php
reconnect:
  enabled: true
  initial_delay: 1s
  max_delay: 60s
  backoff_multiplier: 2
scripts:
  - ~/.config/praetor/scripts
commands:
  default_delay: 900ms
  min_interval: 400ms
  max_queue_size: 20
  high_priority: []
ui:
  sidebar_open: true
  default_tab: all
  scrollback: 5000
  sidebar_width: 40
  minimap_scale: 0.8
  minimap_height: 12
  quick_cycle_modes:
    - disable
  color_words: false
  echo_commands: true
  hide_ips: false
  custom_tabs: []
highlights: []
notifications:
  desktop:
    health_below:
      enabled: true
      threshold: 25
    fatigue_below:
      enabled: false
      threshold: 10
    patterns: []
logging:
  app:
    level: info
    max_size_mb: 5
  session:
    enabled: true
    path: ""
```

## Testing

Tests across the project:
- `internal/engine/` — unit tests covering Lua VM, pattern matching, command queue, timers, metrics, persistent state
- `internal/protocol/` — SKOOT parsing, HTML parsing
- `internal/session/` — WebSocket, auth, keyring, reconnection
- `internal/config/` — YAML loading with defaults
- `internal/colorwords/` — color word detection, adjectives, suffixes, plurals, rainbow
- `internal/client/` — session logging


## Known Limitations

- Reconnection UI feedback is limited to status bar text
- Text selection requires hiding the sidebar (Alt+S) or holding Shift
- Lighting level strings are approximate (tuning in progress)
