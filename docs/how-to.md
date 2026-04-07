# How-To Guide

A quick reference for common Praetor tasks. All menu items are accessed by pressing **Esc** to open the menu.

---

## Add Your Local Scripts Directory

Script directories tell Praetor where to find your Lua automation modes and libraries.

1. Press **Esc** to open the menu
2. Select **Script Directories** (under Scripts)
3. Press **A** to add a new directory
4. Type the full path (e.g., `~/my-scripts`)
5. Press **Enter** to confirm
6. Press **Esc** to save and close

You can also edit `~/.config/praetor/config.yaml` directly:

```yaml
scripts:
  - ~/.config/praetor/scripts
  - ~/my-scripts
```

After adding a directory, select **Reload Scripts** from the menu to load the new modes.

---

## Create a "Think/OOC" Channel

> For custom tab filters, these will match any line that contains the filter you specify.

Custom tabs let you filter game text into separate channels. To create a tab that captures think and OOC messages:

1. Press **Esc** to open the menu
2. Select **Custom Tabs** (under Display)
3. Navigate to **+ Add new tab...** and press **Enter**
4. Type `Think/OOC` and press **Enter**
5. You're now in the rule editor — navigate to **+ Add match...** and press **Enter**
6. Type `thinks aloud` and press **Enter**
7. Add another match: `You think aloud`
7. Add another match: `?m OOC>`
8. Press **Esc** to go back to the tab list, then **Esc** again to save

Rules use wildcard patterns (`*` matches any characters, `?` matches a single character) and are case-insensitive substring matches. A line appears in the tab if it matches any include rule and does not match any exclude rule.

Controls in the rule editor:
- **Space** — enable/disable a rule
- **T** — toggle between MATCH and EXCLUDE
- **D** — delete a rule
- **Esc** — back

---

## Create a "Combat/Skills" Channel

1. Press **Esc** → **Custom Tabs** → **+ Add new tab...** → type `Combat` → **Enter**
2. Add match rules for the text you want to capture, for example:
   - `[Success:`
   - `*You hit*`
   - `*You miss*`
   - `*damages you*`
   - `*You begin to*`
   - `*skill in*`
3. Press **Esc** twice to save

Text matching any include rule will appear in both the All tab and your custom tab.

---

## Setup Priority Commands

Priority commands skip to the front of the command queue, executing before other queued commands. Useful for defensive or emergency actions.

1. Press **Esc** to open the menu
2. Select **Priority Commands** (under Scripts)
3. Press **A** to add a command
4. Type the exact command string (e.g., `flee`)
5. Press **Enter** to confirm
6. Press **Esc** to save

You can also set them in config:

```yaml
commands:
  high_priority:
    - stand
    - fall back
```

---

## Setup Loot Highlighting

String highlighting makes rare drops and important text stand out with colored backgrounds.

1. Press **Esc** to open the menu
2. Select **Highlights** (under Display)
3. Navigate to **+ Add new highlight...** and press **Enter**
4. Type the text to match (e.g., `retalq`) and press **Enter**
5. The highlight is created with the **gold** style by default

Controls in the highlights manager:
- **Space** — toggle a highlight on/off
- **S** — cycle the color style (red → gold → green → blue)
- **D** or **Delete** — remove a highlight
- **Esc** — save and close

Available styles:
| Style | Appearance |
|-------|------------|
| red | Red background, white text |
| gold | Yellow background, black text |
| green | Green background, black text |
| blue | Blue background, white text |

Matching is case-insensitive. Patterns are plain text substrings, not wildcards.

---

## Add or Remove Quick-Cycle Modes

Quick-cycle lets you rapidly switch between automation modes with **Alt+M**.

1. Press **Esc** to open the menu
2. Select **Quick-Cycle Modes** (under Scripts)
3. Use **Up/Down** to navigate the list of available modes
4. Press **Space** or **Enter** to toggle a mode on/off
5. Press **Esc** to save

Selected modes cycle in the order they appear in the list. Pressing **Alt+M** advances to the next mode. Press **Alt+X** to disable all automation.

---

## Enable/Disable Colorwords

Colorwords automatically renders color names (e.g., "crimson", "emerald green", "azure") to an approximation of their actual color within game text.

1. Press **Esc** to open the menu
2. Select **Colorwords: ON/OFF** (under Display)
3. Press **Enter** to toggle

---

## Enable/Disable Echo Commands

When enabled, commands you type (and commands sent by scripts) are echoed back in the output window as italic text.

1. Press **Esc** to open the menu
2. Select **Echo Commands: ON/OFF** (under Display)
3. Press **Enter** to toggle

---

## Enable/Disable Hiding IP Addresses

IP masking replaces real IP addresses in game text with randomized fake addresses. Useful when streaming or recording.

1. Press **Esc** to open the menu
2. Select **Hide IP Addresses: ON/OFF** (under Display)
3. Press **Enter** to toggle

Each real IP is consistently mapped to the same fake IP for the session.

---

## Enable Auto-Reconnect

> Important Note: Autoreconnect will dump you right back into the game if you sleep from the WA.

Auto-reconnect automatically attempts to restore your connection when it drops, using exponential backoff.

1. Press **Esc** to open the menu
2. Select **Auto Reconnect: ON/OFF** (under Connection)
3. Press **Enter** to toggle

---

## Find Your Game Session Logs

Session logs record timestamped game text to a file.

1. Enable logging: **Esc** → **Game Logs: ON/OFF** (under Logs)
2. View or change the log directory: **Esc** → **Log Location** (under Logs)
   - Press **Enter** to edit the path, or leave empty for the default
   - Default location: `~/.config/praetor/logs/`
3. Log files are named `session_YYYY-MM-DD_HH-MM-SS.log`

Each session creates a new log file. Entries are formatted as `[HH:MM:SS] text`.

---

## Export Persistent Metrics Data

Lua scripts can persist data across sessions (e.g., armor absorption tracking, kill counts). You can view and export this data.

1. Press **Esc** to open the menu
2. Select **Persistent Data** (under Data)
3. Use **Up/Down** to navigate keys
4. Press **Space** to select keys for export
5. Press **Enter** to export selected keys to JSON
6. Exports are saved to `~/.config/praetor/exports/`

To clear data: select keys with **Space**, then press **D** and confirm with **Y**.

---

## Search Game Help Files

Praetor can send help queries directly to the game server.

1. Type `/help` to open the help screen
2. Press **S** to enter search mode
3. Type your search term (e.g., `combat`) and press **Enter**
4. Praetor sends `?combat` to the game server, and the response appears in your output

You can also press **W** from the help screen to open the [TEC Wiki](https://eternal-city.wikidot.com) in your browser.

---

## Use Quick-Keys

These key bindings are always available during gameplay:

| Key | Action |
|-----|--------|
| **Tab** / **Shift+Tab** | Next / previous tab |
| **Alt+1** through **Alt+9**, **Alt+0** | Jump directly to a tab (0 = 10th) |
| **Alt+S** | Toggle the sidebar (minimap, compass, status bars) |
| **Alt+M** | Cycle to the next automation mode |
| **Alt+X** | Disable all automation |
| **Esc** | Open the menu |
| **Ctrl+C** | Clear input line, or press twice to quit |
| **PgUp** / **PgDn** | Scroll output |
| **Mouse wheel** | Scroll output (3 lines per tick) |
| **Enter** (empty input) | Send a blank line to the server |
