<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# WezTerm: instant per-pane mode toggle

failsafe resolves the current pane's mode from `~/.claude/pane-mode/$WEZTERM_PANE`
(see the mode-source chain in the README). WezTerm's Lua API exposes the same id via
`pane:pane_id()`, so the toggle can **write that file directly** — no subprocess, no
`failsafe` call, instant. Missing file = `read` (the safe default).

Bind `Ctrl+Alt+T` to flip the focused pane between `read` and `read & write`.

## Drop-in snippet

Paste into your `wezterm.lua` (or a module you `require`). Self-contained — no
dependencies beyond WezTerm itself.

```lua
local wezterm = require("wezterm")
local act = wezterm.action

-- ~/.claude/pane-mode/<pane_id> holds "read" or "read & write".
-- This must match failsafe's mode-source chain ($WEZTERM_PANE).
local function mode_dir()
  return (os.getenv("HOME") or "") .. "/.claude/pane-mode"
end

local function mode_path(pane_id)
  return mode_dir() .. "/" .. tostring(pane_id)
end

local function get_mode(pane_id)
  local f = io.open(mode_path(pane_id), "r")
  if not f then return "read" end          -- missing file = safe default
  local line = f:read("*l")
  f:close()
  if line and #line > 0 then return line end
  return "read"
end

local function toggle_mode(pane_id)
  local current = get_mode(pane_id)
  local next_mode = (current == "read") and "read & write" or "read"
  os.execute("mkdir -p '" .. mode_dir() .. "'")
  local f = io.open(mode_path(pane_id), "w")
  if f then
    f:write(next_mode)                       -- canonical value the Rego policies match
    f:close()
  end
  return current, next_mode
end

-- Ctrl+Alt+T: flip the focused pane and toast the change.
local toggle_action = wezterm.action_callback(function(window, pane)
  local old, new = toggle_mode(pane:pane_id())
  window:toast_notification("🔒 failsafe", old .. "  →  " .. new, nil, 3000)
  -- nudge the tab bar to rerender the badge (below)
  window:set_config_overrides(window:get_config_overrides() or {})
end)

return {
  keys = {
    { key = "t", mods = "CTRL|ALT", action = toggle_action },
  },
  get_mode = get_mode,   -- exported so the badge below can read it
}
```

Wire the keybinding into your config:

```lua
local toggler = dofile(wezterm.config_dir .. "/failsafe_toggle.lua") -- if you saved it as a module
local config = wezterm.config_builder()
config.keys = config.keys or {}
for _, k in ipairs(toggler.keys) do
  table.insert(config.keys, k)
end
```

## Optional: a tab-title badge (`r` / `rw`)

Show the focused pane's mode in the tab title so the state is always visible.

```lua
wezterm.on("format-tab-title", function(tab)
  local mode = toggler.get_mode(tab.active_pane.pane_id)
  local badge = (mode == "read & write") and " rw " or " r "
  return {
    -- amber when writable (caution), dim when read-only
    { Foreground = { Color = (mode == "read & write") and "#ffb02e" or "#7a756c" } },
    { Text = badge },
    { Text = tab.active_pane.title },
  }
end)
```

## Make it yours: "sudo mode"

Flipping a pane to `read & write` is failsafe's `sudo` — you're handing the agent the
sharp knives, on purpose, for a moment. Lean into it so the elevated state is impossible
to miss.

```lua
-- "sudo make me a sandwich."  — https://xkcd.com/149/
local toggle_action = wezterm.action_callback(function(window, pane)
  local old, new = toggle_mode(pane:pane_id())
  if new == "read & write" then
    window:toast_notification("🔓 failsafe: sudo mode", "write enabled — with great power…", nil, 4000)
  else
    window:toast_notification("🔒 failsafe", "back to read-only. phew.", nil, 2500)
  end
  window:set_config_overrides(window:get_config_overrides() or {})
end)
```

Badge it as `sudo` when elevated:

```lua
local badge = (mode == "read & write") and " ⚡ sudo " or " r "
```

**Bonus — sudo timeout.** Real `sudo` forgets you after a few minutes; a *fail*-safe
should too. Auto-revert a pane to read-only after N minutes of write, so you never walk
away with the knives out:

```lua
-- inside toggle_mode, right after writing "read & write":
if next_mode == "read & write" then
  os.execute(string.format(
    "( sleep 600; echo read > '%s' ) >/dev/null 2>&1 &",  -- 10 min, detached
    mode_path(pane_id)))
end
```

(The label is cosmetic — the file still stores the canonical `read & write`, so policies
and `failsafe mode get` are unaffected.)

## Notes

- The file always stores the **canonical** value (`read` / `read & write`) because
  that is what the bundled Rego policies match on (`input.mode == "read"`). The CLI's
  `rw` / `ro` aliases (`failsafe mode set rw`) normalize to the same canonical value, so
  the WezTerm toggle and the CLI stay perfectly compatible.
- Prefer not to write the file from Lua? Replace `toggle_mode` with a spawn of
  `failsafe toggle` — but the direct write is instant and needs no binary on PATH.
