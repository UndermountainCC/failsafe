load helpers

setup()    { setup_sandbox; need failsafe; }
teardown() { teardown_sandbox; }

# tmux.md states the chain order WEZTERM_PANE -> TMUX_PANE -> ITERM_SESSION_ID.
@test "chain order: WEZTERM_PANE wins over TMUX_PANE (black-box)" {
  write_mode_file "%w" "read & write"
  write_mode_file "%t" "read"
  WEZTERM_PANE="%w" TMUX_PANE="%t" run failsafe mode get
  [ "$status" -eq 0 ]
  [[ "$output" == "read & write"* ]]
  [[ "$output" == *"/pane-mode/%w"* ]]
}

# All four docs: "missing file = read (safe default)".
@test "missing mode file resolves to read" {
  WEZTERM_PANE="%none" run failsafe mode get
  [[ "$output" == read* ]]
}

# All four docs: the canonical value is what the bundled Rego policies match.
@test "every rego mode comparison is exactly input.mode == \"read\"" {
  run grep -rhoE 'input\.mode == "[^"]*"' "$DOCS_REPO_ROOT"/internal/embed/policies/*.rego
  [ "$status" -eq 0 ]
  while IFS= read -r line; do
    [ "$line" = 'input.mode == "read"' ]
  done <<< "$output"
}

# Docs claim rw/ro aliases normalize to the canonical bytes written to the file.
@test "mode set aliases write canonical bytes" {
  for a in rw w "read & write"; do
    write_mode_file "%a" "read"
    WEZTERM_PANE="%a" failsafe mode set "$a"
    [ "$(read_mode_file "%a")" = "read & write" ]
  done
  for a in ro r read; do
    write_mode_file "%a" "read & write"
    WEZTERM_PANE="%a" failsafe mode set "$a"
    [ "$(read_mode_file "%a")" = "read" ]
  done
}

# claude-statusline.sh relies on `failsafe mode get | cut -f1` — assert the value
# is the first tab-delimited field.
@test "mode get output is tab-delimited (value in field 1)" {
  write_mode_file "%w" "read & write"
  WEZTERM_PANE="%w" run bash -c 'failsafe mode get | cut -f1'
  [ "$output" = "read & write" ]
}
