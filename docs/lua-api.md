# Lua API Reference

Praetor provides a Lua 5.1 scripting environment (via [gopher-lua](https://github.com/yuin/gopher-lua)) for automating gameplay. Scripts are loaded from configurable directories.

## Script Structure

A **mode** is a Lua file that returns a table with `reactions` and optionally `on_start`/`on_stop`:

```lua
local M = {}

function M.on_start(args)
    -- Called when the mode is activated via /mode <name> [args]
    -- args is a table of strings from the command
end

function M.on_stop()
    -- Optional. Called when the mode is deactivated.
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
