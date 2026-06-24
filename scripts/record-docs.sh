#!/usr/bin/env bash
# Copyright 2026 Undermountain Coding Company
# SPDX-License-Identifier: Apache-2.0
#
# record-docs.sh — drive the failsafe CLI under virtui and emit asciicast
# recordings for the documentation site.
#
# This is the *recording* half of the docs automation flow. It is a sibling to
# the doc-validation harness (test/docs/, PR #2): that harness proves the
# copy-paste snippets in docs/ actually run against the real binary; this script
# produces the demo recordings that *show* them running, from the same commands,
# so the two never drift.
#
# It is deliberately scoped to the HEADLESS CLI surface (explain / mode / audit /
# report). The GUI terminal-integration surfaces (WezTerm toasts & tab-title,
# iTerm2's Python keybinding) are NOT recordable headlessly and stay manual —
# see docs/demos/README.md, same honest split as test/docs/REPORT.md.
#
# Requirements (all optional — missing tools downgrade gracefully, never fail):
#   - virtui      https://github.com/honeybadge-labs/virtui  (the recorder)
#   - agg         https://github.com/asciinema/agg           (.cast -> .gif)   [optional render]
#   - svg-term    npm i -g svg-term-cli                       (.cast -> .svg)   [optional render]
#
# Usage:
#   scripts/record-docs.sh              # record all scenarios into docs/assets/casts/
#   scripts/record-docs.sh --render     # also render .cast -> .svg/.gif if a renderer is present
#   COLS=100 ROWS=24 scripts/record-docs.sh
set -euo pipefail

# ── Resolve paths ────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="${OUT_DIR:-$REPO_ROOT/docs/assets/casts}"
COLS="${COLS:-92}"
ROWS="${ROWS:-22}"
RENDER=0
[[ "${1:-}" == "--render" ]] && RENDER=1

mkdir -p "$OUT_DIR"

log()  { printf '\033[1;34m▸\033[0m %s\n' "$*" >&2; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*" >&2; }
skip() { printf '\033[1;33mSKIP\033[0m %s\n' "$*" >&2; exit 0; }

# ── Preflight: virtui ────────────────────────────────────────────────────────
command -v virtui >/dev/null 2>&1 || skip \
  "virtui not found — install from https://github.com/honeybadge-labs/virtui (go install …/cmd/virtui@latest). No recordings produced."

# ── Build the binary under test into a throwaway bindir ──────────────────────
BINDIR="$(mktemp -d)"
log "building failsafe -> $BINDIR/failsafe"
( cd "$REPO_ROOT" && go build -o "$BINDIR/failsafe" ./cmd/failsafe )
export PATH="$BINDIR:$PATH"

# ── Sandbox HOME so demos never touch the real ~/.config/failsafe ────────────
# failsafe resolves all of its state from $HOME (policy.rego, trusted-repos.yaml,
# decisions.jsonl, pane-mode files). A throwaway HOME keeps demos side-effect
# free, mirroring helpers.bash in the doc-validation harness. The path is FIXED
# (not mktemp-random) so the cwd printed by `explain` is readable and identical
# on every re-record — committed SVGs then diff only when behaviour changes.
SANDBOX_HOME="${TMPDIR:-/tmp}/failsafe-docs-home"
rm -rf "$SANDBOX_HOME"
export HOME="$SANDBOX_HOME"
mkdir -p "$SANDBOX_HOME/project"
unset FAILSAFE_MODE FAILSAFE_LOG 2>/dev/null || true

# ── virtui daemon lifecycle ──────────────────────────────────────────────────
DAEMON_STARTED=0
cleanup() {
  [[ "$DAEMON_STARTED" == 1 ]] && virtui daemon stop >/dev/null 2>&1 || true
  rm -rf "$BINDIR" "$SANDBOX_HOME"
}
trap cleanup EXIT

log "starting virtui daemon"
virtui daemon start >/dev/null 2>&1 && DAEMON_STARTED=1 || warn "daemon may already be running"

# ── record_scenario NAME -- "cmd|wait-for-text" ["cmd|wait" …] ───────────────
# Spawns a recorded bash session, normalises the prompt, runs each step typing
# the command and waiting for the given screen text, then exits to flush the cast.
record_scenario() {
  local name="$1"; shift
  [[ "$1" == "--" ]] && shift
  local cast="$OUT_DIR/$name.cast"
  log "recording $name -> $cast"

  # `virtui run` prints a human block ("session_id: <id>\npid: …"); pull the id
  # out of it so we don't take a hard dependency on jq.
  local sid
  sid="$(virtui run --record --record-path "$cast" --cols "$COLS" --rows "$ROWS" bash \
         | sed -n 's/^session_id: //p')"
  [[ -n "$sid" ]] || { warn "could not start session for $name"; return 1; }

  # Quiet, deterministic prompt; start from the sandbox project dir.
  virtui exec "$sid" "export PS1='\$ '; cd \"$HOME/project\"; clear" --wait-stable >/dev/null

  local step cmd want
  for step in "$@"; do
    cmd="${step%%|*}"
    want="${step#*|}"
    if [[ "$want" == "$step" || -z "$want" ]]; then
      virtui exec "$sid" "$cmd" --wait-stable >/dev/null
    else
      virtui exec "$sid" "$cmd" --wait "$want" >/dev/null
    fi
  done

  virtui exec "$sid" "exit" --wait-stable >/dev/null 2>&1 || true
  virtui kill "$sid" >/dev/null 2>&1 || true   # finalize/flush the recording
}

# ── Scenarios — the headless CLI demos ───────────────────────────────────────
# Each scenario maps to a doc page. Keep the commands identical to the snippets
# the doc-validation harness checks, so the recording and the prose can't drift.

# 1. The money shot: the guard blocking a destructive command (tutorials/getting-started, README).
record_scenario "explain-block" -- \
  "failsafe explain \"kubectl --context arn:aws:eks:us-east-1:123456789012:cluster/prod delete ns payments\"|Decision: BLOCK"

# 2. The read/write mode switch (reference/modes, tutorials/getting-started step 4).
record_scenario "mode-toggle" -- \
  "failsafe mode get|read" \
  "failsafe mode set rw|read & write" \
  "failsafe mode get|read & write" \
  "failsafe mode set ro|read"

# 3. The policy chain in force (how-to/repo-policy, reference).
record_scenario "audit" -- \
  "failsafe audit|block rule"

log "done — casts in $OUT_DIR"
ls -1 "$OUT_DIR"/*.cast 2>/dev/null >&2 || true

# ── Optional render: .cast -> embeddable .svg / .gif ─────────────────────────
# GitHub markdown and MkDocs do NOT play asciicast inline, so an embeddable
# artifact is required to actually surface a demo in the docs.
if [[ "$RENDER" == 1 ]]; then
  if command -v svg-term >/dev/null 2>&1; then
    for cast in "$OUT_DIR"/*.cast; do
      log "render $(basename "$cast") -> svg"
      svg-term --in "$cast" --out "${cast%.cast}.svg" --window --no-cursor
    done
  elif command -v agg >/dev/null 2>&1; then
    for cast in "$OUT_DIR"/*.cast; do
      log "render $(basename "$cast") -> gif"
      agg "$cast" "${cast%.cast}.gif"
    done
  else
    warn "--render requested but neither svg-term nor agg found; left .cast files only."
  fi
fi
