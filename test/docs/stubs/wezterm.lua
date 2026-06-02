-- Minimal stand-in for the `wezterm` module so the doc snippet loads under plain lua.
local M = {}
M.action = setmetatable({}, { __index = function() return function() end end })
function M.action_callback(fn) return fn end   -- so keys[].action == the raw callback
function M.on(_, _) end
function M.config_builder() return {} end
M.config_dir = os.getenv("HOME") or "."
return M
