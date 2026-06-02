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

@test "read mode renders the lock + read" {
  write_mode_file "sess1" "read"
  run run_sl "${JSON/\%CWD\%//tmp/x}"
  [[ "$output" == "failsafe 🔒 read"* ]]
}

@test "read & write mode renders the open lock + write" {
  write_mode_file "sess1" "read & write"
  run run_sl "${JSON/\%CWD\%//tmp/x}"
  [[ "$output" == "failsafe 🔓 write"* ]]
}

@test "with jq, cwd is tilde-substituted and model appended" {
  need jq
  write_mode_file "sess1" "read"
  run run_sl "${JSON/\%CWD\%/$HOME/code/infra}"
  [[ "$output" == *"~/code/infra"* ]]
  [[ "$output" == *"Opus"* ]]
}

@test "without jq it still prints the guard mode (graceful degrade)" {
  write_mode_file "sess1" "read & write"
  local p; p="$(make_nojq_path)"
  run env PATH="$p" "$SL" <<< "${JSON/\%CWD\%//tmp/x}"
  [[ "$output" == "failsafe 🔓 write"* ]]
  # Prove the jq-enrichment branch was actually skipped (no cwd appended), not
  # merely that the guard label survived.
  [[ "$output" != *"/tmp/x"* ]]
}

@test "output is a single line" {
  write_mode_file "sess1" "read"
  run run_sl "${JSON/\%CWD\%//tmp/x}"
  [ "${#lines[@]}" -eq 1 ]
}
