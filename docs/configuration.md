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

## Credentials

```yaml
credentials:
  backend: keyring                  # keyring, encrypted_file, or disabled
  encrypted_file:
    path: ""                        # empty = state/credentials/credentials.enc
    key_env: PRAETOR_CREDENTIALS_KEY
```

`keyring` is the desktop default. On Linux it requires an available Secret
Service provider on the user-session D-Bus, such as GNOME Keyring or KWallet,
with an unlocked default collection. If the keyring is unavailable, Praetor
shows that condition separately from an empty account list. A login without
**Remember this account** remains available, and a save failure after TEC has
accepted a login does not disconnect or block the game session.

`encrypted_file` is intended for headless web services that cannot safely join
a desktop D-Bus session. `key_env` must contain a base64-encoded 32-byte random
key when Praetor starts. For example:

```sh
openssl rand -base64 32
```

The key is removed from Praetor's environment after the store is initialized;
it is never written to `config.yaml` or beside the encrypted file. The store
uses a versioned AES-256-GCM envelope with a fresh nonce on every write,
authenticated decryption, atomic file replacement, and mode `0600`. The
default parent directory is created with mode `0700`. A missing, malformed, or
incorrect key fails startup rather than falling back to another backend.

`disabled` removes account persistence while retaining ordinary interactive
login. Praetor never silently falls back to plaintext credential storage.
Changing credential backends does not migrate accounts automatically. Add the
accounts again under the new backend after confirming the prior store is
backed up or no longer needed.

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
  default_delay: 1s               # Delay between queued commands
  min_interval: 500ms             # Minimum time between any two sends
  max_queue_size: 20              # Maximum commands in queue
  semicolon_delay_ms: 900         # Delay between ;; commands (100-10000)
  unbusy_delay_ms: 100            # Delay after && unbusy response (0-10000)
  high_priority: []               # Commands that jump to front of queue
  variables:                      # Typed-input substitutions
    target: scarred bandit
```

High priority commands can be configured via Esc → Priority Commands. When a high-priority command is queued, it's inserted at the front (after other high-priority items) instead of the back.

### PraetorScript

Variables can also be managed via Esc → **Variables** or in the sidebar's
**Variables** tab. Reference one as `${name}` in a typed command, such as
`attack ${target}`. Add a literal fallback after a colon, such as
`${count:25}`: a non-empty saved `count` wins, while a missing or empty one
inserts `25`. References are case-sensitive, and saved values and fallbacks are
substituted once rather than recursively. An unknown reference without a
fallback, or any malformed reference, rejects the entire input line without
sending any part of it. Use `\${` to send a literal `${`.

Separate multiple typed commands with `;;` for fixed pacing or `&&` to wait for
roundtime to end. For example, `stand&&climb wall;;look` sends `stand`, waits
for an unbusy response plus the configured 100 ms default before sending
`climb wall`, then waits the configured `;;` delay (900 ms by default) before
sending `look`. Change either delay under Esc → Display & Behavior → Settings.
Each unbusy response advances one pending chain in submission order.
Recognized responses mirror `praetor-scripts`: no longer busy, no longer
stunned, wield/grab/already-wielding confirmations, successful training, and
stopping walking.

A single `;` or `&` remains ordinary text. `\;;` and `\&&` send literal
separators. Praetor splits the line before substituting variables, so a
variable value containing either separator cannot create extra commands. These
features apply to single-line command-input submissions and Action-set buttons.

Use `$()` control steps as complete steps within those chains:

```text
$(wait 2.5)                                      # pause for seconds
$(wait-for "The latch clicks")                   # wait for a future matching line
$(wait-for "The gate opens" timeout 30)          # cancel the chain after 30 seconds
$(wait-for "open" cancel-on "locked" timeout 30)
$(notify "Training complete")                    # notify, then continue
$(notify "Training" "Complete")                  # custom title and message
$(repeat "climb wall" until "You reach the top")
$(repeat "climb wall" until "You reach the top" cancel-on "You fall")
$(repeat "climb wall" until "You reach the top" max 10)
$(repeat "search" count 5)
$(repeat "search" count ${tries:5} cancel-on "You find nothing")
```

Each directive occupies one chain step, so compose it with the existing
separators—for example, `look;;$(wait 2);;inventory`. Separators around a
directive retain their normal configured delay or unbusy behavior; an explicit
`wait` adds its duration at that point in the chain. The safety exception is a
`wait-for` immediately after a sent command: Praetor arms that matcher before
sending the command so a fast response cannot be missed; the separator there
serves as syntax rather than delaying matcher registration.

`wait-for` can optionally use `cancel-on "text"`, `timeout seconds`, or both.
The cancellation text and timeout stop the entire chain instead of advancing
it. A timeout must be positive. Without either clause, the wait remains active
until it matches or the user presses Stop.

`repeat` requires a game command (not a local `/` command), sends it
immediately, then reacts after each recognized unbusy response and the
configured unbusy delay. The `until "text"` form sends the command again until
its case-sensitive success substring appears; that success advances the
surrounding chain. `max N` counts the initial send as attempt 1 and cancels the
chain rather than sending attempt N+1. Without `max`, retries remain unbounded.

The `count N` form sends the command exactly N times. The initial send is count
1, and each remaining send follows an unbusy response and the configured
unbusy delay. After the final send, one final unbusy response and delay advances
the surrounding chain. Count may use a variable or fallback, such as
`count ${tries:5}`. It is intentionally distinct from `until ... max N`: reaching
an exact count completes normally, while reaching `max` without success cancels
the chain as an error. Either repeat form may use `cancel-on "text"`, which
stops the whole chain without advancing. Matching is case-sensitive substring matching. Text
arguments may contain separators because quoted strings are parsed before the
outer chain. Escape a quote or backslash inside them as `\"` or `\\`. Use
`\$(` to send a literal `$(`. Variables work in directive arguments and, like
the rest of the line, are validated before anything is sent.

Variables are read afresh every time either is invoked, so edits take effect
immediately. Variables also apply to multiline input and `/send` files, but
those paths do not interpret command-chain separators. Other sidebar buttons,
numpad movement, Lua scripts, and `/play` playback bypass typed-input
processing. A single line is limited to 100 commands across both separator
types and control steps. Pending chains are discarded on disconnect. In the
GUI, a dedicated row below the input keeps PraetorScript status on the left and
the Play/Stop and mode controls on the right. The status slot remains visible
as an idle placeholder when no chain is running; an active chain fills it with
the active step, wait/retry state, remaining timeout, and count of additional
concurrent chains, and replaces Play with Stop. A `;;` pacing step shows
its original configured delay for the whole pause rather than counting down;
explicit waits and timed `wait-for` steps count down in whole seconds, rounded
up so a new 30-second wait begins at `30s`. Stop cancels queued
commands, explicit waits, substring reactions, and repeats.

## UI

```yaml
ui:
  display_mode: sidebar           # TUI: sidebar | topbar | off; GUI: sidebar | off
  default_tab: all                # Initial tab: all, metrics
  scrollback: 5000                # Lines of scrollback per tab
  sidebar_width: 40               # TUI sidebar width in columns
  gui_sidebar_width: 260          # GUI sidebar width in pixels (180-600)
  minimap_scale: 1.0              # Minimap zoom multiplier (0.5-3.0)
  minimap_height: 12              # TUI minimap height in terminal rows
  gui_minimap_height: 160         # GUI map height in pixels (80-400)
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
  keep_input_on_send: false       # GUI: retain and select the last sent command
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

All other UI toggles are available via the Esc menu and persisted when you press Save.

`input_spellcheck` controls the native spellchecker for the command textarea.
On Linux, Praetor enables WebKitGTK spellchecking with the first usable locale
from `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, or `LANG` (falling back to `en_US`).
The Linux packages include the English Hunspell dictionary; source builds need
a Hunspell/Enchant dictionary installed for the selected locale.

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
    allow_script_notifications: false # Permit Lua notify() desktop alerts and GUI toasts
    sound: false                   # Play the OS default sound for every notification
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

Desktop notifications use the system's native notification mechanism
(`notify-send` on Linux, `osascript` on macOS, PowerShell toast on Windows).
`sound` is a single global switch for notification audio. When enabled, every
desktop notification requests the OS-managed default alert sound; when
disabled, Praetor suppresses notification sounds. System volume, notification
preferences, and Do Not Disturb settings still take precedence.

`allow_script_notifications` is off by default. Enable it to permit Lua
scripts' `notify()` calls to create desktop alerts and in-app notification
toasts. Health, fatigue, and text-pattern notifications remain controlled by
their own settings.

## Logging

```yaml
logging:
  app:
    level: info                   # debug, info (default), warn, error
    max_size_mb: 5                # Size of each app-log segment
    retain: false                 # Keep dated app-log segments permanently
  session:
    enabled: true                 # Record game session transcripts
    path: ""                      # Empty = ~/.config/praetor/logs/
```

- **App logs** normally use `~/.local/state/praetor/tec.log` with one
  size-rotated `tec.log.1` backup. When `retain` is enabled, each
  application start creates `tec_YYYY-MM-DD_HH-MM-SS.log`; reaching
  `max_size_mb` opens another collision-safe dated/suffixed segment. Praetor
  never deletes retained segments.
- **Session logs** record timestamped game text to `~/.config/praetor/logs/` (or the configured path).

Session-log settings are available via Esc → Game Logs and Esc → Log Location.
The desktop/web Settings dialog also provides **Retain application logs**. That
setting changes the application-log writer at process startup, so restart
Praetor after changing it.

### What the app log records at each level

The app log level controls how much detail the current `tec.log` or retained
`tec_*.log` captures:

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

Retained application logs are created with owner-only permissions (`0600`).
At debug level they are substantially more sensitive than session transcripts.
Retention is deliberately opt-in and has no age or space limit:
monitor the state volume, include the files in backups only when intended, and
delete or archive them under an external policy if disk growth becomes a
concern. Existing `tec.log`/`.1` files are left in place when retained mode is
enabled, and retained files are not collapsed back into `tec.log` when it is
disabled.

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

## Onboarding

```yaml
onboarding:
  welcome_shown: false             # Internal one-time GUI welcome marker
```

On a new installation, Praetor sets this to `true` when the first successful
GUI login displays the welcome popup. The popup links to the Praetor overview,
guide, and scripts wiki pages; it never opens a page until the user clicks a
link. Existing configs that predate this setting are treated as already
welcomed so an upgrade does not trigger the popup. The GUI-only `/guide`
command reopens the same popup without changing this marker.

## File Locations

| Data | Location |
|------|----------|
| Config | `~/.config/praetor/config.yaml` |
| Scripts (default) | `~/.config/praetor/scripts/` |
| Session logs | `~/.config/praetor/logs/` |
| Exports | `~/.config/praetor/exports/` |
| Notes | `~/.config/praetor/notes/` |
| App logs | `~/.local/state/praetor/tec.log` and `.1`, or retained `tec_YYYY-MM-DD_HH-MM-SS*.log` |
| Persistent state | `~/.local/share/praetor/persistent_state.json` |
| Credentials | System keyring, or an explicitly configured encrypted file under the state directory |
| Automatic web TLS | `~/.local/state/praetor/tls/praetor-web-self-signed.{crt,key}` |

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
