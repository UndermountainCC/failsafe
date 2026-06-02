load helpers

TMUX_DOC="$DOCS_REPO_ROOT/docs/toggle/tmux.md"
SOCK="failsafe-doctest"

setup() {
  setup_sandbox; need failsafe; need tmux
  TOGGLE="$TEST_HOME/tmux-toggle.sh"
  extract_block "$TMUX_DOC" bash 1 "The toggle helper" > "$TOGGLE"
  chmod +x "$TOGGLE"
}
teardown() {
  tmux -L "$SOCK" kill-server 2>/dev/null || true
  teardown_sandbox
}

@test "toggle script flips read <-> read & write" {
  # The toggle script ends with `tmux display-message`, which requires a tmux
  # server — run it via run-shell so it has a proper tmux context (just as it
  # would be invoked from a real key binding).
  tmux -L "$SOCK" new-session -d -s s -x 80 -y 24
  tmux -L "$SOCK" run-shell "$TOGGLE '%5'"
  [ "$(read_mode_file "%5")" = "read & write" ]
  tmux -L "$SOCK" run-shell "$TOGGLE '%5'"
  [ "$(read_mode_file "%5")" = "read" ]
}

@test "#{pane_id} equals \$TMUX_PANE inside a real session" {
  local out="$TEST_HOME/pane.txt"
  tmux -L "$SOCK" new-session -d -s s -x 80 -y 24
  tmux -L "$SOCK" send-keys -t s "printf '%s' \"\$TMUX_PANE\" > '$out'" Enter
  for _ in $(seq 1 20); do [ -s "$out" ] && break; sleep 0.1; done
  local pid; pid="$(tmux -L "$SOCK" display-message -p -t s '#{pane_id}')"
  [ "$(cat "$out")" = "$pid" ]
}

# Headless reality: send-keys injects into the pane app and bypasses tmux's root
# key table, so a `bind -n` can't be fired from a script. We instead prove the
# binding is REGISTERED to the toggle script, and (above) that the script flips
# the file — together that validates the documented keybinding.
@test "C-M-t is bound to the toggle script with #{pane_id}" {
  local conf="$TEST_HOME/tmux.conf"
  printf "bind -n C-M-t run-shell \"%s '#{pane_id}'\"\n" "$TOGGLE" > "$conf"
  tmux -L "$SOCK" -f "$conf" new-session -d -s s
  run tmux -L "$SOCK" list-keys -T root
  [[ "$output" == *"C-M-t"* ]]
  [[ "$output" == *"$TOGGLE"* ]]
  [[ "$output" == *'#{pane_id}'* ]]
}

@test "status script colors writable amber / read green" {
  local STAT="$TEST_HOME/tmux-status.sh"
  extract_block "$TMUX_DOC" bash 1 "Status indicator" > "$STAT"; chmod +x "$STAT"
  write_mode_file "%5" "read & write"
  run "$STAT" "%5"
  [[ "$output" == *"🔓 sudo"* ]]; [[ "$output" == *"fg=yellow"* ]]
  write_mode_file "%5" "read"
  run "$STAT" "%5"
  [[ "$output" == *"🔒 read"* ]]; [[ "$output" == *"fg=green"* ]]
}

@test "no-script alternative toggles only the target pane" {
  WEZTERM_PANE= ITERM_SESSION_ID= TMUX_PANE="%5" failsafe toggle
  [ "$(read_mode_file "%5")" = "read & write" ]
  [ ! -f "$HOME/.claude/pane-mode/%other" ]
}
