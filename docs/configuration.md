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

## Reconnection

```yaml
reconnect:
  enabled: true                   # Auto-reconnect on disconnect
  initial_delay: 1s
  max_delay: 60s
  backoff_multiplier: 2           # Exponential backoff between attempts
```

Toggleable via Esc → Auto Reconnect.

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
  echo_commands: true             # Show sent commands in output
  hide_ips: false                 # Scramble IP addresses in text
  custom_tabs: []                 # User-defined tabs (managed via menu)
```

All UI toggles are available via the Esc menu and saved automatically.

### Custom Tabs

```yaml
  custom_tabs:
    - name: Combat
      visible: true
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

## File Locations

| Data | Location |
|------|----------|
| Config | `~/.config/praetor/config.yaml` |
| Scripts (default) | `~/.config/praetor/scripts/` |
| Session logs | `~/.config/praetor/logs/` |
| Exports | `~/.config/praetor/exports/` |
| App logs | `~/.local/state/praetor/tec.log` |
| Persistent state | `~/.local/share/praetor/persistent_state.json` |
| Credentials | System keyring |
