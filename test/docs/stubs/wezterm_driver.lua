-- wezterm_driver.lua <pane_id>
-- Loads stubs/wezterm.lua as the `wezterm` module, dofiles the extracted snippet,
-- fires its first keybinding action with fake window/pane, prints the resulting mode.
local stub_path = assert(os.getenv("STUB"), "STUB unset")
package.preload["wezterm"] = function() return dofile(stub_path) end

local mod = dofile(assert(os.getenv("SNIPPET"), "SNIPPET unset"))
local id = assert(arg[1], "pane id arg required")

local pane = { pane_id = function() return id end }
local win = {
  toast_notification = function() end,
  get_config_overrides = function() return {} end,
  set_config_overrides = function() end,
}
mod.keys[1].action(win, pane)     -- runs toggle_mode(id), writes the mode file
io.write(mod.get_mode(id))         -- canonical value the snippet reads back
