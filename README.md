# Praetor

A terminal-based game client for [The Eternal City](https://www.eternalcitygame.com/), built with Go. Replaces the browser-based Orchil client with a native terminal experience.

## Features

- **Kitty Graphics** — pixel-accurate minimap and compass rendered inline via Kitty graphics protocol
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

## Requirements

- Go 1.25+ (for building from source)
- A terminal with [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/) support (Kitty, WezTerm, Ghostty) for minimap/compass rendering
- Linux, macOS, or Windows

## Quick Start

```bash
# Clone and build
git clone https://github.com/cyber-godzilla/praetor.git
cd praetor
make build

# Run
./praetor
```

On first launch, Praetor creates a default config at `~/.config/praetor/config.yaml` and a scripts directory at `~/.config/praetor/scripts/`.

## Installation

### Homebrew (macOS / Linux)

```bash
brew install cyber-godzilla/tap/praetor
```

### Apt (Debian / Ubuntu)

```bash
# Add the GPG key
curl -fsSL "https://packages.buildkite.com/cybergodzilla-2099/praetor-debian/gpgkey" | \
  sudo gpg --dearmor -o /etc/apt/keyrings/praetor-archive-keyring.gpg

# Add the repository
echo "deb [signed-by=/etc/apt/keyrings/praetor-archive-keyring.gpg] https://packages.buildkite.com/cybergodzilla-2099/praetor-debian/any/ any main" | \
  sudo tee /etc/apt/sources.list.d/praetor.list

# Install
sudo apt update && sudo apt install praetor
```

### Yum (Fedora / RHEL / CentOS)

```bash
# Add the repository
sudo tee /etc/yum.repos.d/praetor.repo <<'EOF'
[praetor]
name=Praetor
baseurl=https://packages.buildkite.com/cybergodzilla-2099/praetor-rpm/rpm_any/rpm_any/$basearch
enabled=1
repo_gpgcheck=1
gpgcheck=0
gpgkey=https://packages.buildkite.com/cybergodzilla-2099/praetor-rpm/gpgkey
priority=1
EOF

# Install
sudo yum install praetor
```

### GitHub Releases

Download pre-built binaries from the [releases page](https://github.com/cyber-godzilla/praetor/releases).

### From Source

```bash
git clone https://github.com/cyber-godzilla/praetor.git
cd praetor
make build
sudo mv praetor /usr/local/bin/
```

### Build Options

```bash
make build    # Build binary
make run      # Build and run
make test     # Run all tests
make check    # Run vet + fmt + tests
make clean    # Remove binary
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
