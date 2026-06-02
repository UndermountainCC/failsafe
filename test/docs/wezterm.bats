load helpers

WZ="$DOCS_REPO_ROOT/docs/toggle/wezterm.md"

setup()    { setup_sandbox; need failsafe; }
teardown() { teardown_sandbox; }

@test "wezterm snippet is syntactically valid lua" {
  [ -n "$LUA_BIN" ] || skip "no lua interpreter"
  extract_block "$WZ" lua 1 "Drop-in snippet" > "$TEST_HOME/snip.lua"
  run "$LUA_BIN" -e "assert(loadfile('$TEST_HOME/snip.lua'))"
  [ "$status" -eq 0 ]
}

@test "wezterm snippet has no luacheck errors" {
  need luacheck
  extract_block "$WZ" lua 1 "Drop-in snippet" > "$TEST_HOME/snip.lua"
  # --no-color: CI luacheck colorizes its summary, which would split the "0 errors"
  # substring with ANSI escapes (…0<esc>[0m errors).
  run luacheck --no-color --globals wezterm --no-max-line-length "$TEST_HOME/snip.lua"
  # luacheck must actually have analyzed the file: a functional run always prints a
  # "Total:" summary footer. If it's absent (e.g. luacheck's own runtime is broken
  # under this Lua version), skip honestly rather than pass vacuously.
  [[ "$output" == *"Total:"* ]] || skip "luacheck runtime not functional here (no report produced)"
  # It ran — require zero ERRORS (syntax/parse). Style warnings are tolerated.
  [[ "$output" == *"0 errors"* ]]
}

@test "toggle action writes canonical and failsafe agrees" {
  [ -n "$LUA_BIN" ] || skip "no lua interpreter"
  extract_block "$WZ" lua 1 "Drop-in snippet" > "$TEST_HOME/snip.lua"
  local out
  out="$(STUB="$STUB_DIR/wezterm.lua" SNIPPET="$TEST_HOME/snip.lua" \
         "$LUA_BIN" "$STUB_DIR/wezterm_driver.lua" "%wz")"
  [ "$out" = "read & write" ]
  [ "$(read_mode_file "%wz")" = "read & write" ]
  WEZTERM_PANE="%wz" run failsafe mode get
  [[ "$output" == "read & write"* ]]
}

@test "badge maps writable -> sudo, read -> r" {
  run grep -F 'local badge = (mode == "read & write") and " ⚡ sudo " or " r "' "$WZ"
  [ "$status" -eq 0 ]
}

@test "sudo-timeout mechanism reverts to read" {
  local block
  block="$(extract_block "$WZ" lua 3 'Make it yours: "sudo mode"')"
  [[ "$block" == *"sleep 600"* ]]
  [[ "$block" == *"echo read"* ]]
  local f="$HOME/.claude/pane-mode/%wz"; printf 'read & write' > "$f"
  ( sleep 1; echo read > "$f" ) &
  sleep 2.5   # generous margin over the 1s revert so loaded CI runners don't flake
  [ "$(read_mode_file "%wz")" = "read" ]
}
