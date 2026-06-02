load ../helpers

# LIVE GUI-app check — NOT run in CI (needs the real WezTerm binary installed).
# Run locally via `make validate-docs-live`.
#
# We can't drive a GUI keypress, but `wezterm show-keys` loads a real config file
# (no window needed) and prints the resolved key table. This proves the doc's
# snippet actually loads in WezTerm and registers the Ctrl+Alt+t binding.

WZ="$DOCS_REPO_ROOT/docs/toggle/wezterm.md"

setup()    { setup_sandbox; }
teardown() { teardown_sandbox; }

@test "wezterm loads the doc snippet and registers the Ctrl+Alt+t binding" {
  need wezterm
  extract_block "$WZ" lua 1 "Drop-in snippet" > "$TEST_HOME/failsafe_toggle.lua"
  # Wire it exactly as the doc's "Wire the keybinding into your config" step shows.
  cat > "$TEST_HOME/wezterm.lua" <<EOF
local wezterm = require('wezterm')
local toggler = dofile('$TEST_HOME/failsafe_toggle.lua')
local config = wezterm.config_builder()
config.keys = config.keys or {}
for _, k in ipairs(toggler.keys) do table.insert(config.keys, k) end
return config
EOF
  run wezterm --config-file "$TEST_HOME/wezterm.lua" show-keys
  [ "$status" -eq 0 ]                       # config loaded without error
  # The binding for the bare 't' key must carry both Ctrl and Alt and fire a
  # callback (wezterm renders the action_callback as EmitEvent("user-defined-N")).
  local kt
  kt="$(printf '%s\n' "$output" | grep -E '[[:space:]]t[[:space:]]+->')"
  [ -n "$kt" ]
  [[ "$kt" == *ALT* ]]
  [[ "$kt" == *CTRL* ]]
  [[ "$kt" == *EmitEvent* || "$kt" == *user-defined* ]]
}
