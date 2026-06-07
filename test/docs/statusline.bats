load helpers

SL="$DOCS_REPO_ROOT/examples/claude-statusline.sh"
JSON='{"cwd":"%CWD%","model":{"display_name":"Opus"}}'

setup()    { setup_sandbox; need failsafe; export CLAUDE_SESSION_ID="sess1"; }
teardown() { teardown_sandbox; }

run_sl() { # $1 = json; pipes it into the real script
  printf '%s' "$1" | "$SL"
}

@test "statusline script exists and is bash" {
  [ -f "$SL" ]
  head -1 "$SL" | grep -q 'bash'
}

@test "enabled mode renders the lock + enabled" {
  write_mode_file "sess1" "enabled"
  run run_sl "${JSON/\%CWD\%//tmp/x}"
  [[ "$output" == "failsafe 🔒 enabled"* ]]
}

@test "disabled mode renders the open lock + disabled" {
  write_mode_file "sess1" "disabled"
  run run_sl "${JSON/\%CWD\%//tmp/x}"
  [[ "$output" == "failsafe 🔓 disabled"* ]]
}

@test "with jq, cwd is tilde-substituted and model appended" {
  need jq
  write_mode_file "sess1" "enabled"
  run run_sl "${JSON/\%CWD\%/$HOME/code/infra}"
  [[ "$output" == *"~/code/infra"* ]]
  [[ "$output" == *"Opus"* ]]
}

@test "without jq it still prints the guard mode (graceful degrade)" {
  write_mode_file "sess1" "disabled"
  local p; p="$(make_nojq_path)"
  run env PATH="$p" "$SL" <<< "${JSON/\%CWD\%//tmp/x}"
  [[ "$output" == "failsafe 🔓 disabled"* ]]
  # Prove the jq-enrichment branch was actually skipped (no cwd appended), not
  # merely that the guard label survived.
  [[ "$output" != *"/tmp/x"* ]]
}

@test "output is a single line" {
  write_mode_file "sess1" "enabled"
  run run_sl "${JSON/\%CWD\%//tmp/x}"
  [ "${#lines[@]}" -eq 1 ]
}
