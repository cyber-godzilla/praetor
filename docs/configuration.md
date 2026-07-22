# Configuration Reference

Praetor stores its configuration at `~/.config/praetor/config.yaml`. A default config is created on first launch. Most settings can also be changed via the pause menu (Esc).

## Server

```yaml
server:
  host: game.eternalcitygame.com
  port: 8080
  protocol: ws                    # ws or wss
  login_url: https://login.eternalcitygame.com/login.php
```

## Scripts

```yaml
scripts:
  - ~/.config/praetor/scripts
  - ~/my-custom-scripts
  - $HOME/git/community-scripts
```

List of directories to load Lua scripts from. Each directory is added to Lua's `package.path` (for `require()`) and scanned for mode files.

Supports `~` and `$ENV_VAR` expansion. If empty, defaults to `~/.config/praetor/scripts/`.

Manageable via Esc → Script Directories.

## Commands

```yaml
commands:
  default_delay: 900ms            # Delay between queued commands
  min_interval: 400ms             # Minimum time between any two sends
  max_queue_size: 20              # Maximum commands in queue
  high_priority: []               # Commands that jump to front of queue
```

High priority commands can be configured via Esc → Priority Commands. When a high-priority command is queued, it's inserted at the front (after other high-priority items) instead of the back.

## UI

```yaml
ui:
  sidebar_open: true              # Sidebar visible on start
  default_tab: all                # Initial tab: all, metrics
  scrollback: 5000                # Lines of scrollback per tab
  sidebar_width: 40               # Sidebar width in columns
  minimap_scale: 0.8              # Minimap zoom level
  minimap_height: 12              # Minimap height in terminal rows
  quick_cycle_modes:              # Modes cycled by Alt+M
    - disable
  color_words: false              # Color word highlighting
  echo_typed_commands: true       # Echo commands you type
  echo_script_commands: true      # Echo commands sent by Lua scripts
  hide_ips: false                 # Scramble IP addresses in text
  input_spellcheck: true          # GUI: spellcheck the command input (webview native)
  mobile_output_font_size: 14     # Web: output size for mobile-width layouts
  mobile_show_toolbar: true       # Web: show Actions / Modes / Menu on mobile
  mobile_show_tab_bar: true       # Web: show the tab selector on mobile
  mobile_hide_navigation_on_input: false # Web: hide map/compass while typing
  mobile_lowercase_first_letter: false   # Web: counter keyboard capitalization
  custom_tabs: []                 # User-defined tabs (managed via menu)
```

The web Settings modal keeps desktop and mobile output sizes independent. The
mobile size accepts values from 6 through 40 CSS pixels; desktop output retains
its existing 8 through 40 range. Existing configurations without
`mobile_output_font_size` inherit their current `output_font_size` once, so an
upgrade does not unexpectedly change text size. The remaining `mobile_*`
fields are checkboxes. Mobile settings affect only the browser mobile layout;
the TUI and native Wails UI ignore them. The toolbar and tab selector default
to visible, while navigation hiding and command normalization default to
disabled. When the tab selector is hidden, its Menu button moves to the far
right of the mobile status row.

All other UI toggles are available via the Esc menu and saved automatically.

### Custom Tabs

```yaml
  custom_tabs:
    - name: Combat
      visible: true
      echo_commands: false         # Route command echoes here (exclude-only tabs only)
      rules:
        - pattern: "Success:"
          include: true
          active: true
        - pattern: "You are no longer busy"
          include: true
          active: true
```

Each rule has:
- `pattern` — wildcard pattern (`*` and `?` supported)
- `include` — `true` to include matching lines, `false` to exclude matching lines
- `active` — toggle the rule on/off

`echo_commands` applies only when the tab has no active include rules (i.e. it is exclude-only, including zero-rule catch-all tabs). When false, command echoes are not routed to the tab even though other non-excluded text is.

Managed via Esc → Custom Tabs.

## Highlights

```yaml
highlights:
  - pattern: "rare item"
    style: gold                   # red, gold, green, blue
    active: true
```

Case-insensitive substring matching. Highlighted text appears with colored background in the output. Managed via Esc → Highlights.

## Notifications

```yaml
notifications:
  desktop:
    health_below:
      enabled: true
      threshold: 25               # Notify when health drops below this %
    fatigue_below:
      enabled: false
      threshold: 10
    patterns:                     # Custom text pattern notifications
      - pattern: "rare drop"
        title: "Loot Alert"
        message: ""               # Empty = use matched text
        enabled: true
```

Desktop notifications use the system's native notification mechanism (`notify-send` on Linux, `osascript` on macOS, PowerShell toast on Windows).

## Logging

```yaml
logging:
  app:
    level: info                   # debug, info, warn, error
    max_size_mb: 5                # Log rotation size
  session:
    enabled: true                 # Record game session transcripts
    path: ""                      # Empty = ~/.config/praetor/logs/
```

- **App logs** are written to `~/.local/state/praetor/tec.log` with size-based rotation.
- **Session logs** record timestamped game text to `~/.config/praetor/logs/` (or the configured path).

Log settings are available via Esc → Game Logs and Esc → Log Location.

### What the app log records at each level

The app log level controls how much detail `tec.log` captures:

- **`info` (default)** — lifecycle and operational messages (connect/disconnect,
  auth results, mode changes, errors). The game transcript and your typed input
  are **not** recorded here, so the app log is not a second hidden transcript.
- **`debug`** — everything at `info` plus the full received/sent traffic
  (`[RECV:*]`/`[SEND:*]`). Use this only when diagnosing a problem: it includes
  everything you type, which can contain an accidental password paste. Handshake
  `SECRET` lines are always redacted.
- **`warn`/`error`** — progressively quieter; note these hide the operational
  `info` messages that are useful for support.

The **session transcript** (Esc → Game Logs) is the user-controlled game log and
is unaffected by the app log level — it records exactly as configured.

## Transport security

The default `server.protocol: ws` and the game's `login_url` determine whether
traffic is encrypted:

- `ws://` sends the session cookies, the MD5 handshake, and all game traffic in
  the clear; an `http://` login URL POSTs your password unencrypted.
- `wss://` (and an `https://` login URL) are fully supported and recommended
  **if the game server offers TLS**.

praetor logs a startup warning for each cleartext setting but does not force a
change — the shipped default is `ws://` because the server may not support TLS.
Switch `server.protocol` to `wss` (and `login_url` to `https://…`) if the server
accepts it.

## Updates

```yaml
updates:
  check: true                     # GUI: check GitHub releases at startup
```

When enabled, the desktop GUI makes a single anonymous request to the GitHub
releases API shortly after launch and shows a toast if a newer version exists.
Nothing is downloaded or installed automatically, and failures are silent.
Toggleable in the GUI under Settings → "Check for updates on startup".

## File Locations

| Data | Location |
|------|----------|
| Config | `~/.config/praetor/config.yaml` |
| Scripts (default) | `~/.config/praetor/scripts/` |
| Session logs | `~/.config/praetor/logs/` |
| Exports | `~/.config/praetor/exports/` |
| Notes | `~/.config/praetor/notes/` |
| App logs | `~/.local/state/praetor/tec.log` |
| Persistent state | `~/.local/share/praetor/persistent_state.json` |
| Credentials | System keyring |

## How the config file is written

The GUI and TUI save `config.yaml` **atomically**: the new content is written to
a temporary file in the same directory and then renamed over the original, so a
crash or power loss mid-save never leaves a truncated file that fails to load.
The file's permissions are preserved across saves.

Two caveats:

- **Comments and unknown keys are not preserved.** A UI-triggered save marshals
  the known settings and rewrites the file, so hand-written comments and any
  keys praetor doesn't recognize are dropped. Edit `config.yaml` by hand only
  while the app is closed if you want to keep comments.
- **Multi-instance play is last-writer-wins.** Running one instance per
  character is supported, but if two instances save the shared `config.yaml`,
  the last save wins. The atomic write guarantees the file is never *torn*, only
  that the later writer's version replaces the earlier one.
