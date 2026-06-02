# shellcheck shell=bash
# Shared helpers for the doc-validation bats suite.

# Repo root (this file lives at test/docs/helpers.bash).
DOCS_REPO_ROOT="${DOCS_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

# Snapshot the real HOME before any test sandboxes it (needed for python user-site imports).
ORIG_HOME="${ORIG_HOME:-$HOME}"

# Resolve interpreters once (names vary across distros/brew).
LUA_BIN="$(command -v lua || command -v lua5.4 || command -v luajit || true)"
PYTHON_BIN="$(command -v python3 || command -v python || true)"

EXTRACT_SH="$(dirname "${BASH_SOURCE[0]}")/lib/extract.sh"
STUB_DIR="$(dirname "${BASH_SOURCE[0]}")/stubs"

extract_block() { bash "$EXTRACT_SH" "$@"; }

setup_sandbox() {
  TEST_HOME="$(mktemp -d "${TMPDIR:-/tmp}/failsafe-doctest.XXXXXX")"
  export HOME="$TEST_HOME"
  mkdir -p "$HOME/.claude/pane-mode" "$HOME/.config/failsafe"
  unset WEZTERM_PANE TMUX_PANE ITERM_SESSION_ID KITTY_WINDOW_ID CLAUDE_SESSION_ID FAILSAFE_MODE
}

# Only ever removes dirs created by setup_sandbox (our own template prefix), so a
# stray or externally-set TEST_HOME can never be rm -rf'd.
teardown_sandbox() {
  case "${TEST_HOME:-}" in
    */failsafe-doctest.*) rm -rf "$TEST_HOME" ;;
    *) : ;;
  esac
  return 0
}

# Skip (not fail) when a required tool is missing — CI installs everything, so a
# skip in CI is itself a signal.
need() { command -v "$1" >/dev/null 2>&1 || skip "$1 not installed"; }

write_mode_file() { printf '%s' "$2" > "$HOME/.claude/pane-mode/$1"; }
read_mode_file()  { tr -d '\r\n' < "$HOME/.claude/pane-mode/$1"; }
