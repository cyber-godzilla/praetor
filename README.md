# Praetor

A cross-platform desktop client for [The Eternal City](https://www.eternalcitygame.com/), built with Go. Replaces the browser-based Orchil client.

Praetor's primary application is a **native desktop GUI** (the `praetor` app, built with [Wails](https://wails.io)). It also ships a **secondary terminal client** (`praetor-tui`) for players who prefer a terminal or run headless/over SSH. Both are driven by the same engine, so features and behaviour stay identical.

## Features

- **Minimap & Compass** — pixel-accurate minimap and compass, rendered natively in the desktop GUI and via the Kitty graphics protocol in the terminal client
- **Lua Scripting** — automation engine with pattern matching, command queuing, timers, and persistent state
- **Multiple Script Directories** — load modes and libraries from any number of directories
- **Custom Tabs** — user-defined output tabs with include/exclude pattern filters
- **Colorwords** — automatic color highlighting of color words in game text (200+ colors, adjectives, suffixes, rainbow)
- **String Highlighting** — configurable pattern highlighting for rare loot detection
- **Desktop Notifications** — alerts for health/fatigue thresholds and custom pattern matches
- **Metrics Dashboard** — session tracking for kills, actions, and custom metrics with history
- **Persistent State** — Lua scripts can persist data across sessions (e.g., armor absorption tracking)
- **Multi-Account** — system-keyring or encrypted-service storage with account selection
- **Scrollback & History Search** — Ctrl+F searches the output, Ctrl+R reverse-searches your commands (GUI)
- **Responsive Sidebar** — auto-hides or compacts when terminal is too small
- **IP Masking** — optional scrambling of IP addresses in game text
- **Update Notifier** — the GUI checks GitHub releases at startup and toasts when a newer version exists (optional)
- **Notes** — a freeform notepad (GUI): /notes, /notes add|open|delete|list, or the Notes menu item; each note is a plaintext file in ~/.config/praetor/notes/
- **Shared Web Mode** — one headless process serves the full client to authenticated desktop and mobile browsers on a trusted LAN

The two binaries:

- **`praetor`** — the desktop **GUI** (Wails); the packages install this into your applications menu.
- **`praetor-tui`** — the **terminal** client (Bubbletea), fully supported; needs a terminal with [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/) support (Kitty, WezTerm, Ghostty) for the minimap/compass.

> **Upgrading from an older release?** The `praetor` command used to be the terminal client. It is now the GUI, and the terminal client moved to `praetor-tui`. See the per-manager notes below.

## Requirements

- Linux, macOS, or Windows
- **Terminal client (`praetor-tui`):** Go 1.25+ to build; a terminal with [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/) support (Kitty, WezTerm, Ghostty) for the minimap/compass
- **Desktop GUI (`praetor`):** Go 1.25+ and the [Wails](https://wails.io) toolchain to build from source

## Quick Start

```bash
# Clone
git clone https://github.com/cyber-godzilla/praetor.git
cd praetor

# Terminal client
make build      # produces ./praetor-tui
./praetor-tui

# Desktop GUI
cd gui && make deps && make build   # produces gui/build/bin/praetor
```

On first launch, Praetor creates a default config at `~/.config/praetor/config.yaml` and a scripts directory at `~/.config/praetor/scripts/`.

## Installation

Packages install **both** binaries: `praetor` (GUI) and `praetor-tui` (terminal).

### Homebrew (macOS)

```bash
brew install --cask cyber-godzilla/tap/praetor      # desktop GUI (→ /Applications)
brew install cyber-godzilla/tap/praetor-tui         # terminal client
```

Upgrading from the old terminal `praetor` formula? `brew upgrade` migrates it to the GUI cask automatically (the tap ships a formula→cask migration). If for any reason it doesn't, do it manually:

```bash
brew update
brew upgrade                 # auto-migrates praetor (formula) → praetor (cask)
# fallback if the auto-migration doesn't trigger:
brew uninstall praetor && brew install --cask cyber-godzilla/tap/praetor
```

### Apt (Debian / Ubuntu)

```bash
# Add the GPG key
curl -fsSL "https://packages.buildkite.com/cybergodzilla-2099/praetor-debian/gpgkey" | \
  sudo gpg --dearmor -o /etc/apt/keyrings/praetor-archive-keyring.gpg

# Add the repository
echo "deb [signed-by=/etc/apt/keyrings/praetor-archive-keyring.gpg] https://packages.buildkite.com/cybergodzilla-2099/praetor-debian/any/ any main" | \
  sudo tee /etc/apt/sources.list.d/praetor.list

# Install (provides both `praetor` GUI and `praetor-tui`)
sudo apt update && sudo apt install praetor
```

The GUI appears in your desktop applications menu after install. **Existing users** who had the old terminal `praetor` upgrade in place with `sudo apt update && sudo apt upgrade` — the `praetor` command becomes the GUI and `praetor-tui` is added alongside it. Both amd64 and arm64 are built.

### Yum (Fedora / RHEL / CentOS)

```bash
# Add the repository
sudo tee /etc/yum.repos.d/praetor.repo <<'EOF'
[praetor]
name=Praetor
baseurl=https://packages.buildkite.com/cybergodzilla-2099/praetor-rpm/rpm_any/rpm_any/$basearch
enabled=1
repo_gpgcheck=0
gpgcheck=0
priority=1
EOF

# Install
sudo yum install praetor
```

### Chocolatey (Windows)

```powershell
# Add the repository (one-time)
choco source add -n=praetor -s="https://packages.buildkite.com/cybergodzilla-2099/praetor-nuget/nuget/index.json"

# Install (provides both `praetor` GUI and `praetor-tui`)
choco install praetor
```

The GUI is added to the Start Menu after install.

### GitHub Releases

Download pre-built binaries from the [releases page](https://github.com/cyber-godzilla/praetor/releases).

### From Source

```bash
git clone https://github.com/cyber-godzilla/praetor.git
cd praetor
make build                       # produces ./praetor-tui
sudo mv praetor-tui /usr/local/bin/
```

For the GUI, see [`gui/`](gui/) (`cd gui && make deps && make build`).

### Build Options

```bash
make build    # Build the terminal client (./praetor-tui)
make run      # Build and run the terminal client
make test     # Run all tests
make check    # Run vet + fmt + tests
make clean    # Remove built binaries
```

## Usage

### Login

Praetor shows a splash screen, then either an account selection screen (if you have stored credentials) or a login form. Desktop installs use the system keyring by default; headless services can explicitly use an encrypted credential file with a separately managed key. Credential-storage failures never block an otherwise successful TEC connection, and Praetor never falls back to plaintext storage.

### Key Bindings

| Key | Action |
|-----|--------|
| Tab / Shift+Tab | Next / previous tab |
| Alt+1..9, Alt+0 | Jump to tab (0 = 10th) |
| Alt+S | Toggle sidebar |
| Alt+M | Quick-cycle automation mode |
| Esc | Open menu |
| Ctrl+C | Clear input / confirm quit |
| Ctrl+F | Search scrollback (GUI) |
| Ctrl+R | Search command history (GUI) |
| PgUp / PgDn | Scroll output |
| Mouse wheel | Scroll output |
| Enter (empty) | Send blank line to server |

### Slash Commands

| Command | Description |
|---------|-------------|
| `/mode <name> [args]` | Set automation mode (alias `/sm`) |
| `/list` | List / select available modes |
| `/toggle <label>` | Toggle a boolean state value |
| `/set <label> <val>` | Set a state value |
| `/wiki [term]` | Open a TEC wiki bookmark (bare `/wiki` lists them) |
| `/maps [term]` | Open a TEC map page (bare `/maps` lists them) |
| `/calc` | Rank-bonus & training-cost calculator (alias `/rb`) |
| `/kudos [name] [msg]` | Manage kudos favorites / queued messages |
| `/notes` | Freeform notepad — add / open / delete / list (GUI) |
| `/help` | Show the help screen |

### Menu (Esc)

The pause menu provides access to all settings:

- **Scripts** — Reload scripts, manage script directories, configure quick-cycle modes, set priority commands
- **Display** — Highlights, custom tabs, colorwords, echo commands, IP masking
- **Tools & References** — Notes, kudos, rank-bonus calculator, wiki & map bookmarks
- **Logs** — Game log toggle and log location
- **Data** — Persistent data viewer with export/clear

## How-To Guide

See [docs/how-to.md](docs/how-to.md) for step-by-step guides on setting up custom tabs, highlights, script directories, quick-cycle modes, and more.

## Configuration

Config file: `~/.config/praetor/config.yaml`

See [docs/configuration.md](docs/configuration.md) for the full configuration reference.

## Lua Scripting

Praetor loads Lua scripts from configurable directories. Scripts can automate gameplay, track metrics, and persist data across sessions.

See [docs/lua-api.md](docs/lua-api.md) for the complete Lua API reference.

## Scripts

Community scripts are available at [praetor-scripts](https://github.com/cyber-godzilla/praetor-scripts).

To install:

```bash
git clone https://github.com/cyber-godzilla/praetor-scripts.git ~/praetor-scripts
```

Then add the directory via Esc → Script Directories, or in your config:

```yaml
scripts:
  - ~/praetor-scripts
```

## License

[GPL v3](LICENSE)
