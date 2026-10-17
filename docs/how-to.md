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

## Create a "Think/OOC" View

> Custom View filters match any line that contains the filter you specify.

Custom Views let you filter game text into separate channels and place them in
a main tab or any UUID-addressed sidebar. To create one that captures think and
OOC messages:

1. Press **Esc** to open the menu
2. Select **Layout**
3. Under **Filtered text components**, enter `Think/OOC` and press **+**
4. Drag the component into a sidebar, or unplace it and use that sidebar's
   **Add component by ID** picker
5. Add match rules for `thinks aloud`, `You think aloud`, and `?m OOC>`
6. Close the editor; placement and filter changes save immediately

Rules use case-insensitive wildcard patterns: `*` matches any characters, `?`
matches one character, and `\*` or `\?` match literal wildcard characters.
They may use live variables such as `${target}`. A line appears in the tab if it
matches any include rule and does not match any exclude rule; unresolved or
empty expanded rules match nothing.

Each rule can be enabled, changed between **matches** and **excludes**, or
deleted directly in the editor.

---

## Create a "Combat/Skills" View

1. Press **Esc** → **Layout**, add a Custom View named `Combat`, and choose its location
2. Add match rules for the text you want to capture, for example:
   - `[Success:`
   - `*You hit*`
   - `*You miss*`
   - `*damages you*`
   - `*You begin to*`
   - `*skill in*`
3. Press **Save**

Text matching any include rule will appear in both All and your Custom View.

---

## Setup Priority Commands

Priority commands skip to the front of the command queue, executing before other queued commands. Useful for defensive or emergency actions.

1. Press **Esc** to open the menu
2. Select **High-Priority Commands** under **Automation**
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

Matching is case-insensitive and supports `*` and `?` wildcards, escaped literal
wildcards (`\*`, `\?`), and live variables such as `${loot}`. The exact text
consumed by each match is highlighted.

---

## Highlight Wounds in Character Descriptions

Praetor can color verified wound phrases by injury category and severity. The
feature starts off so upgrades do not change existing output.

In the GUI or web client:

1. Open **Menu → Settings**
2. Turn on **Highlight wounds**
3. Choose a family for cuts, bruises/fractures, punctures/wounds, and burns
4. Save

In the TUI:

1. Press **Esc**
2. Select **Wound Highlighting** under Display
3. Toggle **Highlight wounds** and use Left/Right to choose each family
4. Press **Esc** to save

Severity chooses the shade automatically, and levels 4–5 are bold. This version
recognizes combat-result phrases such as `a shallow cut`, `a major bruise`, and
`a gaping wound`, plus captured character-inspection descriptions such as `His
right arm is badly scratched` and `Her face is completely black and blue`.

The inspection severity mapping is provisional. Bleeding text, plural or
treated wounds, unknown phrases, incomplete healer descriptions, and the
still-unverified level-5 burn description are left unchanged. Completing the
catalogue is tracked as a [0.6.0 release blocker](release-0.6.0.md#wound-language-catalogue-blocking).

---

## Identify Speakers With Speech Colors

Speech Colors tint a character's entire speech line, making speakers easier to
identify while quickly scrolling through conversation.

1. Press **Esc** to open the menu
2. Select **Speech Colors** under **Text Parsing and Display**
3. Select **+ Add character** and enter a character name or pattern. Patterns
   match the whole name, support `*` and `?` wildcards, and can use current
   command variables such as `${speaker}` or `${speaker:Apula}`
4. Open the Colorword-style dropdown and search by partial color name, or type a
   family such as `red` to browse all reds
5. Optionally enable **Think** and/or **OOC** for that character
6. Select a style and save

The picker includes ordinary Colorwords and special entries such as alternating
plural colors and rainbow. You can also right-click any recognized speech line
and choose **Set a speech color**; the speaker's name is filled in automatically.

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

Commands can be echoed back in the output window as italic text. The two sources are controlled independently:

- **Echo Typed Commands** — commands you type
- **Echo Script Commands** — commands sent by Lua scripts

1. Press **Esc** to open the menu
2. Select **Echo Typed Commands: ON/OFF** or **Echo Script Commands: ON/OFF** (under Display)
3. Press **Enter** to toggle

**Custom View routing:** Exclude-only Custom Views do not receive command
echoes by default (otherwise they would catch every echo, since exclude rules
match everything not explicitly excluded). Use **Esc → Layout → Custom Views**
to toggle **Route command echoes**. The setting only affects exclude-only views;
views with active match rules already filter echoes through those rules.

---

## Enable/Disable Hiding IP Addresses

IP masking replaces real IP addresses in game text with randomized fake addresses. Useful when streaming or recording.

1. Press **Esc** to open the menu
2. Select **Hide IP Addresses: ON/OFF** (under Display)
3. Press **Enter** to toggle

Each real IP is consistently mapped to the same fake IP for the session.

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
2. In the GUI, enter a topic in **Game help** and click **Search**. In the TUI,
   press **S**, type the topic, and press **Enter**.
3. Praetor sends the query (for example, `?combat`) to the game server, and the
   response appears in your output.

Use **Open TEC Wiki** in the GUI, or press **W** in the TUI, to open the
[TEC Wiki](https://tec-wiki.com) in your browser.

---

## Take Notes (GUI)

The desktop GUI includes a freeform notepad. Each note is a plain text file under `~/.config/praetor/notes/`, so you can also read or back them up with any editor.

1. Type `/notes` (or **Esc** → **Notes**, under Tools & References) to open the Notepad
2. Click **+ New note**, give it a title, write the body, then press **Save**
3. Click any note in the list to edit it; the **✕** on a note deletes it (with a confirm)

Command shortcuts (GUI only):

| Command | Action |
|---------|--------|
| `/notes` | Open the notepad (list view) |
| `/notes add <title>` | Open the editor for a new note titled `<title>` |
| `/notes open <title>` | Open an existing note in the editor |
| `/notes delete <title>` | Delete a note by title |
| `/notes list` | Print your notes (title + preview) into the output |

Notes are shared across all your characters, and the list shows the most-recently-edited first. Closing the editor with unsaved changes prompts you before discarding them.

---

## Use Quick-Keys

These key bindings are always available during gameplay:

| Key | Action |
|-----|--------|
| **Tab** / **Shift+Tab** | Next / previous tab |
| **Alt+1** through **Alt+9**, **Alt+0** | Jump directly to a tab (0 = 10th) |
| **Alt+S** | Toggle the sidebar (GUI); cycle sidebar, top bar, and hidden modes (TUI) |
| **Alt+M** | Cycle to the next automation mode |
| **Alt+X** | Stop sends, queued chains, and scripts (GUI); disable automation (TUI) |
| **Esc** | Open the menu |
| **Ctrl+C** | Clear input line, or press twice to quit |
| **PgUp** / **PgDn** | Scroll output |
| **Mouse wheel** | Scroll output (3 lines per tick) |
| **Enter** (empty input) | Send a blank line to the server |
