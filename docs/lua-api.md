# Lua API Reference

Praetor provides a Lua 5.1 scripting environment (via [gopher-lua](https://github.com/yuin/gopher-lua)) for automating gameplay. Scripts are loaded from configurable directories.

## Script Structure

A **mode** is a Lua file that returns a table with `reactions` and optionally `on_start`/`on_stop`:

```lua
local M = {}

-- Optional metadata, read when the script loads. The GUI uses it to describe
-- the mode as /mode is typed and to annotate the mode picker. See "Mode
-- Metadata" below.
M.usage = '<item> [count]'
M.desc = 'One line describing what the mode does'
M.chains = true
M.hidden = false   -- true keeps the mode out of the hint (it still runs)

function M.on_start(args)
    -- Called when the mode is activated via /mode <name> [args]
    -- args is a table of strings from the command
end

function M.on_stop()
    -- Optional. Called when the mode is deactivated.
    -- The outgoing mode's still-pending queued commands are dropped *before*
    -- on_stop runs, so any send() here (e.g. sheathe, stand) survives the switch
    -- and reaches the server instead of being wiped by the mode change.
end

M.reactions = {
    {
        match = 'pattern',           -- string or table of strings
        action = function(text)      -- called when pattern matches
            send('command')
        end,
        condition = function()       -- optional: only fire if true
            return status.health > 50
        end,
        delay = 500,                 -- optional: ms delay before action
    },
}

return M
```

Mode arguments use double quotes for word grouping. Quotes can wrap a whole
argument or only the value portion of an option: `/mode unlock_all
from:"2 sack"` passes `from:2 sack` as one entry in `args`. Within quoted
text, `\"` represents a literal quote and `\\` represents a literal backslash.

### Mode Metadata

Four optional fields let a mode describe itself to the client. They are read
once at load time — everything a mode registers at runtime (`metrics.track`,
`state.display`) is only known after `on_start` has fired, which is too late to
describe a mode the player has not started yet.

| Field | Type | Meaning |
|---|---|---|
| `usage` | string | Argument signature, without the mode name. Omit when the mode takes no arguments. |
| `desc` | string | One line, sentence case, no trailing period. |
| `chains` | boolean | The mode honors completion handoff suffixes. |
| `hidden` | boolean | Keep the mode out of the command hint. |

Notation for `usage` follows the convention in the scripts repo: `<required>`,
`[optional]`, `[flagword]` for a literal word, `a|b|c` to pick one,
`key:<value>` for a named option, and `[repeatable...]`.

Typing `/mode ` lists every loaded mode; typing part of a name narrows the list;
once the name resolves, the hint shows that mode's own signature and
description, appending the generic
`after_<mode|do|ps>:<mode|command|praetorscript>` suffix when `chains` is set.
Set `chains` only when the mode
genuinely honors completion handoffs — declaring it on a mode that parses them
and then ignores them advertises something that will not happen.

`hidden` suppresses a mode in the hint only, for helpers that are real modes but
noise while typing — an internal route leg, or a mode that exists to be chained
into. It is not access control and not unloading: the mode stays loaded, the
mode picker still lists it, and `/mode <name>` still runs it, because mode
resolution goes through `HasMode`, which never consults this. A hidden mode is
invisible to the hint at every stage, including when its name is typed in full;
the hint falls back to the generic `/mode` signature exactly as it does for a
name that does not exist, so a hidden mode is indistinguishable from an absent
one.

Removing `usage` and `desc` does **not** hide a mode — it still appears as a
bare name with nothing beside it. Only `hidden` removes it.

All of these are descriptive only. Nothing validates arguments against `usage`,
and a field of the wrong type is treated as undeclared rather than failing the
load, so a typo in metadata never costs a working mode. A mode that declares
nothing behaves exactly as it always has.

A **library** is a Lua file loaded via `require()`:

```lua
local S = {}
S.greeting = 'Hello'
S.patterns = {'pattern1', 'pattern2'}
return S
```

## Source trust and isolation

Managed repositories install disabled and **Restricted**. Each enabled
Restricted source runs in its own Lua state, so its globals, library cache, and
standard-library mutations cannot reach another source. Restricted scripts
receive Praetor's documented automation APIs and Lua's base, table, string,
math, and coroutine libraries. Filesystem, process, environment, and debug
access are absent: `os`, `io`, `debug`, `package`, `dofile`, `loadfile`,
`load`, `loadstring`, `getfenv`, `setfenv`, and `collectgarbage` are not
available.

Restricted `require("lib_name")` accepts ordinary Lua module names and searches
only `lib_name.lua` or `lib_name/init.lua` inside that managed source. Absolute
paths, traversal, cross-source imports, native modules, and symlink escapes are
rejected. `print()` is retained for diagnostics, with bounded message length
and rate limiting. Every Lua callback still has Praetor's execution deadline.

A player can explicitly mark a managed source **Trusted** in Script Manager.
Trusted sources share the legacy Lua state and full standard library, including
`os`, `io`, `debug`, and normal `package.path` imports. Existing configured
local directories remain Trusted for backward compatibility. Restricted code
cannot receive a Trusted function, userdata, thread, or metatable through
`state`; portable scalar/table data is copied across state boundaries instead.

## Pattern Matching

The `match` field supports:
- **Literal substrings**: `'You attack'` matches any text containing that string
- **Wildcards**: `'Your * absorbs'` where `*` matches any characters, `?` matches a single character
- **Line anchors**: `'^You attack'` matches only at the start of a line, `'falls.$'` only at the end, and `'^exact text$'` the whole line; anchors combine with wildcards
- **Multiple patterns**: `{'pattern1', 'pattern2'}` matches if any pattern matches

Matching is case-sensitive. The first reaction with a matching pattern wins — subsequent reactions are not checked.

## Functions

### Commands

```lua
send(command)              -- Queue a command to send to the game server
send(command, delay_ms)    -- Queue with a delay in milliseconds
praetor_script(expression) -- Run typed-input PraetorScript (;;, &&, variables, controls)
```

Commands are sent through a queue with configurable delays and minimum intervals. High-priority commands (configured in the menu) jump to the front of the queue. `praetor_script()` schedules its expression outside the current Lua callback and runs it through the same parser as typed single-line input.

### Mode Control

```lua
set_mode(name)             -- Switch to a different mode
set_mode(name, {args})     -- Switch with arguments (passed to on_start)
```

### Notifications

```lua
notify(title, message)     -- Send a shell notification (native desktop or all web clients)
log(message)               -- Write to the application log
```

`notify()` is ignored unless **Allow Script Notifications** is enabled under
Notifications. The permission is off by default.

### Utilities

```lua
random_item(table)         -- Return a random element from an array table
```

## State

Per-mode state that persists across reactions within a session. Cleared on mode switch unless marked persistent.

```lua
state.get(key)             -- Get a value (returns nil if not set)
state.set(key, value)      -- Set a value (string, number, boolean, or table)
state.persist(key)         -- Preserve accumulated long-term data across restarts
state.unpersist(key)       -- Delete a formerly persistent value (migration use)
state.display(key, label)  -- Show this key's value in the sidebar and enable /toggle, /set
state.mode                 -- Read-only: current mode name (string)
```

Ordinary `state` values are private execution details and are cleared when the
mode changes. Do not persist mode steps, arguments, preferences, or workflow
handoff values. Put player-visible durable values in character variables and
pass short-lived handoff values as mode arguments.

### Persistent State

Keys marked with `state.persist(key)` are intended for accumulated long-term
records such as maps, observations, and lifetime statistics. They are:
- Preserved when the mode switches (not cleared)
- Saved to disk (debounced, every 5 seconds)
- Loaded on next app launch

Values can be any Lua type including tables. Persistent data follows the active
character profile (Default uses the authenticated account namespace) and is
stored at `~/.local/share/praetor/persistent_state.json`.

Manage persistent data via Esc → Persistent Data (view, export as JSON, clear).

### Character variables

Lua can read and synchronously write the active character profile's
player-visible variables:

```lua
variables.get(name)         -- String value, or nil when missing
variables.set(name, value)  -- Persist a string before Lua continues
```

These are the same values shown under **Variables** and expanded by `${name}`.
Use them for durable script inputs, preferences, and values a player may need
to inspect or edit when resuming a workflow. Names use the same identifier
rules as `/set-variable`; failed validation or saving raises a Lua error.

## Status

Read-only access to game vitals:

```lua
status.health              -- 0-100
status.fatigue             -- 0-100
status.encumbrance         -- 0-100
status.satiation           -- 0-100
```

## Metrics

Track session metrics that appear in the Metrics tab:

```lua
metrics.track(key, label)  -- Declare a metric with a display label
metrics.inc(key)           -- Increment by 1
metrics.dec(key)           -- Decrement by 1
metrics.set(key, value)    -- Set to a specific integer value
metrics.get(key)           -- Get current value (returns 0 if not tracked)
```

Metrics are per-session. A new session starts each time the mode changes. Session history is displayed in the Metrics tab.

## Timers

```lua
local id = set_timeout(function()
    send('look')
end, 5000)                 -- Fire once after 5 seconds

local id = set_interval(function()
    send('look')
end, 10000)                -- Fire every 10 seconds

clear_timer(id)            -- Cancel a timer
```

All timers are automatically cancelled on mode switch. Repeating intervals must
be at least 10 ms; shorter or non-positive values are rejected instead of being
allowed to create a CPU-saturating loop.

## Time

```lua
time.now()                 -- Current time in milliseconds (Unix epoch)
time.since(timestamp)      -- Milliseconds elapsed since a timestamp
```

## Example: Simple Combat Mode

```lua
local M = {}

function M.on_start(args)
    metrics.track('kills', 'Kills')
    metrics.track('actions', 'Actions')
    state.persist('total_kills')
    send('attack')
end

M.reactions = {
    {
        match = 'You are no longer busy.',
        action = function()
            metrics.inc('actions')
            send('attack')
        end,
    },
    {
        match = 'falls to the ground',
        action = function()
            metrics.inc('kills')
            local total = (state.get('total_kills') or 0) + 1
            state.set('total_kills', total)
        end,
    },
    {
        match = 'You must be standing',
        action = function()
            send('stand')
        end,
    },
}

return M
```
