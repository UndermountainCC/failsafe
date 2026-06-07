load helpers

setup()    { setup_sandbox; }
teardown() { teardown_sandbox; }

@test "setup_sandbox isolates HOME and clears pane vars" {
  [ "$HOME" != "$ORIG_HOME" ]
  [ -d "$HOME/.claude/pane-mode" ]
  [ -z "${WEZTERM_PANE:-}" ] && [ -z "${TMUX_PANE:-}" ]
}

@test "mode-file helpers round-trip" {
  write_mode_file "%5" "disabled"
  [ "$(read_mode_file "%5")" = "disabled" ]
}

@test "need skips when a tool is absent" {
  need definitely-not-a-real-binary-xyz
  false   # unreachable: skip above aborts the test
}

@test "lua resolver finds an interpreter or skips" {
  if [ -z "$LUA_BIN" ]; then skip "no lua interpreter"; fi
  run "$LUA_BIN" -e 'print("ok")'
  [ "$output" = "ok" ]
}
