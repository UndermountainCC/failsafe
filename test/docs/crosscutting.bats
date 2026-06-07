load helpers

setup()    { setup_sandbox; need failsafe; }
teardown() { teardown_sandbox; }

# tmux.md states the chain order WEZTERM_PANE -> TMUX_PANE -> ITERM_SESSION_ID.
@test "chain order: WEZTERM_PANE wins over TMUX_PANE (black-box)" {
  write_mode_file "%w" "disabled"
  write_mode_file "%t" "enabled"
  WEZTERM_PANE="%w" TMUX_PANE="%t" run failsafe mode get
  [ "$status" -eq 0 ]
  [[ "$output" == "disabled"* ]]
  [[ "$output" == *"/pane-mode/%w"* ]]
}

# All four docs: "missing file = enabled (safe default)".
@test "missing mode file resolves to enabled" {
  WEZTERM_PANE="%none" run failsafe mode get
  [[ "$output" == enabled* ]]
}

# All four docs: the bundled Rego policies gate on the boolean failsafe_enabled,
# not on the legacy input.mode string.
@test "bundled rego gates on failsafe_enabled boolean (not input.mode string)" {
  # (a) every deny rule uses the boolean failsafe_enabled flag
  run grep -rn 'not input.failsafe_enabled == false' "$DOCS_REPO_ROOT"/internal/embed/policies/
  [ "$status" -eq 0 ]
  [ -n "$output" ]   # at least one match exists
  # (b) no deny rule compares input.mode == anything
  run grep -rn 'input\.mode ==' "$DOCS_REPO_ROOT"/internal/embed/policies/
  [ "$status" -ne 0 ] || [ -z "$output" ]   # grep found nothing
}

# Back-compat: even though the guard is boolean, mode get still emits a human-readable
# value in field 1 and the file stores the canonical bytes.
@test "mode get field-1 is enabled or disabled (legacy mode back-compat)" {
  write_mode_file "%w" "enabled"
  WEZTERM_PANE="%w" run bash -c 'failsafe mode get | cut -f1'
  [ "$output" = "enabled" ]
  write_mode_file "%w" "disabled"
  WEZTERM_PANE="%w" run bash -c 'failsafe mode get | cut -f1'
  [ "$output" = "disabled" ]
}

# Docs claim rw/ro aliases normalize to the canonical bytes written to the file.
@test "mode set aliases write canonical bytes" {
  for a in rw w off write sudo disabled; do
    write_mode_file "%a" "enabled"
    WEZTERM_PANE="%a" failsafe mode set "$a"
    [ "$(read_mode_file "%a")" = "disabled" ]
  done
  for a in ro r on read lock enabled; do
    write_mode_file "%a" "disabled"
    WEZTERM_PANE="%a" failsafe mode set "$a"
    [ "$(read_mode_file "%a")" = "enabled" ]
  done
}

# claude-statusline.sh relies on `failsafe mode get | cut -f1` — assert the value
# is the first tab-delimited field.
@test "mode get output is tab-delimited (value in field 1)" {
  write_mode_file "%w" "disabled"
  WEZTERM_PANE="%w" run bash -c 'failsafe mode get | cut -f1'
  [ "$output" = "disabled" ]
}
