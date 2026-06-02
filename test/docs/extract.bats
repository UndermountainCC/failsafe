load helpers

EX() { bash "$DOCS_REPO_ROOT/test/docs/lib/extract.sh" "$@"; }

@test "extract.sh returns the first bash block of tmux.md (the toggle script)" {
  run EX "$DOCS_REPO_ROOT/docs/toggle/tmux.md" bash 1 "The toggle helper"
  [ "$status" -eq 0 ]
  [[ "$output" == *"#!/usr/bin/env bash"* ]]
  [[ "$output" == *'printf '"'"'%s'"'"' "$next" > "$file"'* ]]
  [[ "$output" != *"status indicator"* ]]
}

@test "extract.sh anchors to a heading so ordinals are stable" {
  run EX "$DOCS_REPO_ROOT/docs/toggle/tmux.md" bash 1 "Status indicator"
  [ "$status" -eq 0 ]
  [[ "$output" == *"🔓 sudo"* ]]
  [[ "$output" == *"🔒 read"* ]]
}

@test "extract.sh without anchor counts globally" {
  run EX "$DOCS_REPO_ROOT/docs/claude-statusline.md" json 1
  [[ "$output" == *'"statusLine"'* ]]
}
