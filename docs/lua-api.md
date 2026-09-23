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

### Mode Metadata

Four optional fields let a mode describe itself to the client. They are read
once at load time — everything a mode registers at runtime (`metrics.track`,
`state.display`) is only known after `on_start` has fired, which is too late to
describe a mode the player has not started yet.

| Field | Type | Meaning |
|---|---|---|
| `usage` | string | Argument signature, without the mode name. Omit when the mode takes no arguments. |
| `desc` | string | One line, sentence case, no trailing period. |
| `chains` | boolean | The mode honors an `after:<mode>` argument. |
| `hidden` | boolean | Keep the mode out of the command hint. |

Notation for `usage` follows the convention in the scripts repo: `<required>`,
`[optional]`, `[flagword]` for a literal word, `a|b|c` to pick one,
`key:<value>` for a named option, and `[repeatable...]`.

Typing `/mode ` lists every loaded mode; typing part of a name narrows the list;
once the name resolves, the hint shows that mode's own signature and
description, appending `[after:<mode>]` when `chains` is set. Set `chains` only
when the mode genuinely honors the token — declaring it on a mode that parses
`after:` and then ignores it advertises something that will not happen.

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

## Pattern Matching

The `match` field supports:
- **Literal substrings**: `'You attack'` matches any text containing that string
- **Wildcards**: `'Your * absorbs'` where `*` matches any characters, `?` matches a single character
- **Multiple patterns**: `{'pattern1', 'pattern2'}` matches if any pattern matches

Matching is case-sensitive. The first reaction with a matching pattern wins — subsequent reactions are not checked.

## Functions

### Commands

```lua
send(command)              -- Queue a command to send to the game server
send(command, delay_ms)    -- Queue with a delay in milliseconds
```

Commands are sent through a queue with configurable delays and minimum intervals. High-priority commands (configured in the menu) jump to the front of the queue.

### Mode Control

```lua
set_mode(name)             -- Switch to a different mode
set_mode(name, {args})     -- Switch with arguments (passed to on_start)
```

### Notifications

```lua
notify(title, message)     -- Send a desktop notification
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
state.persist(key)         -- Mark key to survive mode switches and app restarts
state.display(key, label)  -- Show this key's value in the sidebar and enable /toggle, /set
state.mode                 -- Read-only: current mode name (string)
```

### Persistent State

Keys marked with `state.persist(key)` are:
- Preserved when the mode switches (not cleared)
- Saved to disk (debounced, every 5 seconds)
- Loaded on next app launch

Values can be any Lua type including tables. Persistent data is scoped per-username and stored at `~/.local/share/praetor/persistent_state.json`.

Manage persistent data via Esc → Persistent Data (view, export as JSON, clear).

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

All timers are automatically cancelled on mode switch.

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
