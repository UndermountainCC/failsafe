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

@test "wezterm snippet passes luacheck (no syntax errors)" {
  need luacheck
  extract_block "$WZ" lua 1 "Drop-in snippet" > "$TEST_HOME/snip.lua"
  run luacheck --globals wezterm --no-unused --no-max-line-length "$TEST_HOME/snip.lua"
  [[ "$output" != *"syntax error"* ]]
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
  sleep 1.6
  [ "$(read_mode_file "%wz")" = "read" ]
}
