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
- **Multi-Account** — system keyring storage with account selection, optional credential storage
- **Responsive Sidebar** — auto-hides or compacts when terminal is too small
- **IP Masking** — optional scrambling of IP addresses in game text

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

Upgrading from the old terminal `praetor` formula? Remove it first, since the GUI now claims that name as a cask:

```bash
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

The GUI appears in your desktop applications menu after install.

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

Praetor shows a splash screen, then either an account selection screen (if you have stored credentials) or a login form. After logging in, you're prompted whether to store credentials in your system keyring.

### Key Bindings

| Key | Action |
|-----|--------|
| Tab / Shift+Tab | Next / previous tab |
| Alt+1..9, Alt+0 | Jump to tab (0 = 10th) |
| Alt+S | Toggle sidebar |
| Alt+M | Quick-cycle automation mode |
| Esc | Open menu |
| Ctrl+C | Clear input / confirm quit |
| PgUp / PgDn | Scroll output |
| Mouse wheel | Scroll output |
| Enter (empty) | Send blank line to server |

### Slash Commands

| Command | Description |
|---------|-------------|
| `/mode <name> [args]` | Set automation mode |
| `/list` | List available modes |
| `/toggle <label>` | Toggle a boolean state value |
| `/set <label> <val>` | Set a state value |
| `/reconnect` | Reconnect to game server |
| `/help` | Show help screen |

### Menu (Esc)

The pause menu provides access to all settings:

- **Scripts** — Reload scripts, manage script directories, configure quick-cycle modes, set priority commands
- **Display** — Highlights, custom tabs, colorwords, echo commands, IP masking
- **Connection** — Auto reconnect toggle
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
