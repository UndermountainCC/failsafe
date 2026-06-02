load helpers

@test "harness loads and repo root resolves" {
  [ -n "$DOCS_REPO_ROOT" ]
  [ -f "$DOCS_REPO_ROOT/docs/toggle/tmux.md" ]
  [ -f "$DOCS_REPO_ROOT/examples/claude-statusline.sh" ]
}
