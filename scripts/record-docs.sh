#!/usr/bin/env bash
# Copyright 2026 Undermountain Coding Company
# SPDX-License-Identifier: Apache-2.0
#
# record-docs.sh — drive the failsafe CLI under virtui and emit asciicast
# recordings for the documentation site.
#
# This is the *recording* half of the docs automation flow. It is a sibling to
# the doc-validation harness (test/docs/): that harness proves the
# copy-paste snippets in docs/ actually run against the real binary; this script
# produces the demo recordings that *show* them running, from the same commands,
# so the two never drift.
#
# It covers the HEADLESS CLI surface (explain / mode / audit / report) AND two
# live, keystroke-driven demos:
#   - tmux-hotkey      : the Ctrl+Alt+T per-pane toggle flipping enabled↔disabled
#                        live, re-running the same command to BLOCK then ALLOW.
#   - claude-hook-block: failsafe blocking a kubectl delete *inside Claude Code*,
#                        then the same hotkey flipping the guard without leaving
#                        the Claude TUI. (opt-in: --claude; uses the REAL home so
#                        Claude's Keychain/~/.claude auth works, spends a few
#                        tokens — see record_claude_hook for the safety notes.)
#
# The tmux configs for the live demos are materialised FROM docs/toggle/tmux.md's
# own fenced blocks (via test/docs/lib/extract.sh), so the recording can't drift
# from the documented setup either.
#
# Requirements (all optional — missing tools downgrade gracefully, never fail):
#   - virtui      https://github.com/honeybadge-labs/virtui  (the recorder)
#   - agg         https://github.com/asciinema/agg           (.cast -> .gif)   [optional render]
#   - svg-term    npm i -g svg-term-cli                       (.cast -> .svg)   [optional render]
#   - tmux                                                     (the two live demos)
#   - claude      Claude Code CLI                              (--claude demo only)
#
# Usage:
#   scripts/record-docs.sh                    # record headless + tmux-hotkey demos
#   scripts/record-docs.sh --render           # also render .cast -> .svg/.gif
#   scripts/record-docs.sh --claude           # additionally record the inside-Claude demo
#   scripts/record-docs.sh --claude --render  # both, in any order
#   COLS=100 ROWS=24 TYPE_DELAY=0 scripts/record-docs.sh
set -euo pipefail

# ── Resolve paths ────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="${OUT_DIR:-$REPO_ROOT/docs/assets/casts}"
EXTRACT="$REPO_ROOT/test/docs/lib/extract.sh"
TMUX_DOC="$REPO_ROOT/docs/toggle/tmux.md"
COLS="${COLS:-92}"
ROWS="${ROWS:-22}"
# Pause (seconds) held AFTER each command's output, so playback lingers on the
# result before moving on instead of blurring past it. virtui records in real
# wall-clock time, so a host-side idle gap becomes a visible pause baked into the
# cast (re-rendering can't add it — it must be recorded). STEP_PAUSE=0 = fast.
STEP_PAUSE="${STEP_PAUSE:-2}"
# Per-character delay (seconds) for the human-visible typing effect. Every command
# is typed one glyph at a time via `virtui type` with this gap between glyphs, so
# playback shows a person typing, not text teleporting in. TYPE_DELAY=0 = instant.
# NOTE: this bakes real wall-clock timing into the cast, so casts (and their SVGs)
# differ byte-for-byte on every re-record even when behaviour is identical.
TYPE_DELAY="${TYPE_DELAY:-0.05}"

# ── Arg parsing: --render and --claude, in any order ─────────────────────────
RENDER=0
CLAUDE=0
for arg in "$@"; do
  case "$arg" in
    --render) RENDER=1 ;;
    --claude) CLAUDE=1 ;;
    *) printf 'record-docs.sh: unknown arg %q (ignored)\n' "$arg" >&2 ;;
  esac
done

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

# ── Capture the REAL home BEFORE sandboxing ──────────────────────────────────
# The inside-Claude demo (record_claude_hook) needs it: Claude Code authenticates
# against the macOS Keychain and ~/.claude, which a throwaway HOME would break.
REAL_HOME="$HOME"

# ── Sandbox HOME so demos never touch the real ~/.config/failsafe ────────────
# failsafe resolves all of its state from $HOME (policy.rego, trusted-repos.yaml,
# decisions.jsonl, pane-mode files). A throwaway HOME keeps demos side-effect
# free, mirroring helpers.bash in the doc-validation harness. The path is FIXED
# (not mktemp-random) so the cwd printed by `explain` is readable and identical
# on every re-record.
SANDBOX_HOME="${TMPDIR:-/tmp}/failsafe-docs-home"
rm -rf "$SANDBOX_HOME"
export HOME="$SANDBOX_HOME"
mkdir -p "$SANDBOX_HOME/project"
unset FAILSAFE_MODE FAILSAFE_LOG 2>/dev/null || true

# ── virtui daemon lifecycle ──────────────────────────────────────────────────
DAEMON_STARTED=0
cleanup() {
  [[ "$DAEMON_STARTED" == 1 ]] && virtui daemon stop >/dev/null 2>&1 || true
  tmux -L failsafe-demo kill-server >/dev/null 2>&1 || true
  tmux -L failsafe-claude-demo kill-server >/dev/null 2>&1 || true
  rm -rf "$BINDIR" "$SANDBOX_HOME"
}
trap cleanup EXIT

log "starting virtui daemon"
virtui daemon start >/dev/null 2>&1 && DAEMON_STARTED=1 || warn "daemon may already be running"

# ── type_line SID TEXT ───────────────────────────────────────────────────────
# Type TEXT one character at a time (human-visible), then press Enter. This is
# the whole reason the casts read like someone at a keyboard rather than a script
# pasting. Single chars — including space, ", -, / — go through `virtui type`
# verbatim (verified). TYPE_DELAY=0 makes it instant.
type_line() {
  local sid="$1" text="$2" i ch
  for (( i = 0; i < ${#text}; i++ )); do
    ch="${text:i:1}"
    virtui type "$sid" "$ch" >/dev/null
    [[ "$TYPE_DELAY" == 0 ]] || sleep "$TYPE_DELAY"
  done
  virtui press "$sid" Enter >/dev/null
}

# ── record_scenario NAME -- "cmd|wait-for-text" ["cmd|wait" …] ───────────────
# Spawns a recorded bash session, normalises the prompt, types each command
# char-by-char and waits for the given screen text, then exits to flush the cast.
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

  # Quiet, deterministic prompt; start from the sandbox project dir. (Setup, not
  # part of the visible story — kept as an instant exec, not char-by-char.)
  virtui exec "$sid" "export PS1='\$ '; cd \"$HOME/project\"; clear" --wait-stable >/dev/null

  local step cmd want
  for step in "$@"; do
    cmd="${step%%|*}"
    want="${step#*|}"
    type_line "$sid" "$cmd"
    if [[ "$want" == "$step" || -z "$want" ]]; then
      virtui wait "$sid" --stable >/dev/null
    else
      virtui wait "$sid" --text "$want" >/dev/null
    fi
    sleep "$STEP_PAUSE"   # linger on this command's output before the next one
  done

  virtui exec "$sid" "exit" --wait-stable >/dev/null 2>&1 || true
  virtui kill "$sid" >/dev/null 2>&1 || true   # finalize/flush the recording
}

# ── build_tmux_config CFGDIR CONF ────────────────────────────────────────────
# Materialise the documented tmux integration into CFGDIR + CONF, straight from
# docs/toggle/tmux.md's fenced blocks so the demo can't drift from the docs.
#   - toggle + status helper scripts -> CFGDIR/{tmux-toggle,tmux-status}.sh (+x)
#   - CONF = the doc's key-binding block + status block, with the doc's literal
#     ~/.config/failsafe paths rewritten to CFGDIR (so a temp CFGDIR works), plus
#     demo-only overrides:
#       status-interval 1  — snappier status flip on toggle (doc ships 2)
#       default-command    — force a clean `$ ` bash prompt in the pane (macOS's
#                            default login shell would otherwise pollute it)
build_tmux_config() {
  local cfgdir="$1" conf="$2"
  mkdir -p "$cfgdir"
  "$EXTRACT" "$TMUX_DOC" bash 1 "The toggle helper"   > "$cfgdir/tmux-toggle.sh"
  "$EXTRACT" "$TMUX_DOC" bash 1 "Status indicator"    > "$cfgdir/tmux-status.sh"
  chmod +x "$cfgdir/tmux-toggle.sh" "$cfgdir/tmux-status.sh"

  {
    "$EXTRACT" "$TMUX_DOC" tmux 1 "The key binding"
    "$EXTRACT" "$TMUX_DOC" tmux 1 "Status indicator"
    printf '\n# --- demo-only overrides ---\n'
    printf 'set -g status-interval 1\n'
    printf 'set -g display-time 1500\n'
    # Keycast wrapper (demo polish only): the doc's C-M-t binding is materialised
    # ABOVE, then this later binding for the SAME key wins. It pops up a big,
    # centered "key was pressed" caption (tmux display-popup — YouTube-demo
    # style, far more noticeable than a status-bar toast), then runs the doc's
    # toggle script by its exact path — the toggle path is byte-identical to the
    # doc; the wrapper only ADDS the on-screen key caption. The same real chord
    # (press Escape Ctrl+T) still fires it through tmux's root key table, so the
    # keystroke is genuine, not faked. The two %s args are reused as literal
    # `\"` (escapes the popup's own -E command string) so bash/printf never
    # re-interpret the \n / \033 escapes meant for the popup's inner printf.
    printf "bind -n C-M-t run-shell \"tmux display-popup -w 34 -h 5 -E %sprintf '%s'; sleep 1.1%s; %s/tmux-toggle.sh '#{pane_id}'\"\n" \
      '\"' "\n   \033[1;7m  ⌨  Ctrl + Alt + T  \033[0m\n" '\"' "$cfgdir"
    printf "set -g default-command \"env PS1='\$ ' BASH_SILENCE_DEPRECATION_WARNING=1 bash --norc\"\n"
  } > "$conf"

  # Rewrite the doc's ~/.config/failsafe reference to the real CFGDIR (absolute),
  # so a throwaway CFGDIR resolves and there's no ~ / chain ambiguity.
  sed -i '' "s#~/.config/failsafe#$cfgdir#g" "$conf"
}

# ── Scenarios — the headless CLI demos ───────────────────────────────────────
# Each scenario maps to a doc page. Keep the commands identical to the snippets
# the doc-validation harness checks, so the recording and the prose can't drift.

# 1. The money shot: the guard blocking a destructive command (tutorials/getting-started, README).
record_scenario "explain-block" -- \
  "failsafe explain \"kubectl --context arn:aws:eks:us-east-1:123456789012:cluster/prod delete ns payments\"|Decision: BLOCK"

# 2. The enabled/disabled mode switch (reference/modes, tutorials/getting-started step 4).
# Later steps use --wait-stable (no |text): their words already sit on the
# screen from earlier steps, so a text wait would fire before the output lands.
record_scenario "mode-toggle" -- \
  "failsafe mode get|enabled" \
  "failsafe mode set disabled|disabled" \
  "failsafe mode get" \
  "failsafe mode set enabled"

# 3. The policy chain in force (how-to/repo-policy, reference).
record_scenario "audit" -- \
  "failsafe audit|block rule"

# ── record_tmux_hotkey — the live per-pane toggle demo ───────────────────────
# Ctrl+Alt+T flips the focused pane enabled↔disabled *live*: the same command
# BLOCKs, then (after the chord) ALLOWs, then BLOCKs again — with the status bar
# following along. Uses the sandbox HOME like the headless scenarios.
record_tmux_hotkey() {
  command -v tmux >/dev/null 2>&1 || { warn "tmux not found — skipping tmux-hotkey demo"; return 0; }
  local name="tmux-hotkey" cast="$OUT_DIR/tmux-hotkey.cast"
  local cfgdir="$HOME/.config/failsafe" conf="$HOME/.demo-tmux.conf"
  log "recording $name -> $cast"

  build_tmux_config "$cfgdir" "$conf"
  tmux -L failsafe-demo kill-server >/dev/null 2>&1 || true   # no zombie server

  # -e: the macOS zsh-deprecation banner is printed by this OUTER bash (before
  # tmux's default-command ever runs), so it must be silenced at spawn time.
  local sid
  sid="$(virtui run --record --record-path "$cast" --cols "$COLS" --rows "$ROWS" -e BASH_SILENCE_DEPRECATION_WARNING=1 bash \
         | sed -n 's/^session_id: //p')"
  [[ -n "$sid" ]] || { warn "could not start session for $name"; return 1; }

  virtui exec "$sid" "export PS1='\$ '; clear" --wait-stable >/dev/null
  # Launch tmux on a private socket with the materialised config (setup, exec'd).
  virtui exec "$sid" "tmux -L failsafe-demo -f \"$conf\" new-session" --wait-stable >/dev/null
  virtui wait "$sid" --stable >/dev/null

  # BLOCK: guard is enabled by default.
  type_line "$sid" 'failsafe explain "kubectl delete ns payments"'
  virtui wait "$sid" --text "Decision: BLOCK" >/dev/null
  sleep "$STEP_PAUSE"

  # Chord → disabled. Status flips 🔒 on → 🔓 sudo. Wait on the ASCII "sudo"
  # (the emoji cells render unpredictably; the word is stable).
  virtui press "$sid" Escape Ctrl+T >/dev/null
  virtui wait "$sid" --text "sudo" >/dev/null
  sleep "$STEP_PAUSE"

  # Re-run the SAME command from shell history (real Up + Enter). Now ALLOW.
  virtui press "$sid" Up >/dev/null
  virtui press "$sid" Enter >/dev/null
  virtui wait "$sid" --text "Decision: ALLOW" >/dev/null
  sleep "$STEP_PAUSE"

  # Chord again → back to enabled. Status returns to 🔒 on; "sudo" disappears.
  virtui press "$sid" Escape Ctrl+T >/dev/null
  virtui wait "$sid" --gone "sudo" >/dev/null
  sleep "$STEP_PAUSE"

  type_line "$sid" "exit"                                  # exit inner pane → tmux ends
  virtui exec "$sid" "exit" --wait-stable >/dev/null 2>&1 || true   # exit outer bash
  virtui kill "$sid" >/dev/null 2>&1 || true

  tmux -L failsafe-demo kill-server >/dev/null 2>&1 || true
}

record_tmux_hotkey

# ── record_claude_hook — failsafe blocking a tool call INSIDE Claude Code ─────
# Opt-in (--claude). This one is special:
#   * It runs with the REAL home (HOME=$REAL_HOME) because Claude Code authen-
#     ticates via the macOS Keychain and ~/.claude — a sandbox HOME breaks auth.
#   * It spends a few real tokens (one short turn that hits the PreToolUse hook).
#   * decisions.jsonl is kept clean via FAILSAFE_LOG -> a throwaway file, so the
#     user's real audit log doesn't grow.
#   * The tmux toggle helper still writes a pane-mode file under
#     $REAL_HOME/.claude/pane-mode/<pane_id>; we rm exactly that file on the way
#     out (diffing the dir before/after so we never touch the user's live panes).
record_claude_hook() {
  command -v claude >/dev/null 2>&1 || { warn "claude CLI not found — skipping inside-Claude demo"; return 0; }
  command -v tmux   >/dev/null 2>&1 || { warn "tmux not found — skipping inside-Claude demo"; return 0; }

  warn "claude demo uses your REAL home ($REAL_HOME) for auth and spends a few tokens."
  local name="claude-hook-block" cast="$OUT_DIR/claude-hook-block.cast"
  local projdir logfile cfgdir conf
  projdir="$(mktemp -d)"
  logfile="$(mktemp)"
  cfgdir="$(mktemp -d)"           # NOT the real ~/.config/failsafe
  conf="$cfgdir/demo-tmux.conf"
  log "recording $name -> $cast"

  build_tmux_config "$cfgdir" "$conf"
  tmux -L failsafe-claude-demo kill-server >/dev/null 2>&1 || true

  # The recorded Claude runs with the operator's own real global config, on
  # purpose: this demo must never instruct the inner agent to skip its own
  # review steps or confirmations. If other guards (skills, permission gates)
  # fire before failsafe's hook, the recording shows layered defenses honestly —
  # re-record on a machine with a plainer config if you want the hook-only shot.

  # Snapshot the real pane-mode dir so we can remove only the file our toggle adds.
  local panedir="$REAL_HOME/.claude/pane-mode"
  local before after
  before="$(mktemp)"; after="$(mktemp)"
  mkdir -p "$panedir"
  ( cd "$panedir" && ls -1 ) > "$before" 2>/dev/null || true

  # -e: silence the macOS zsh-deprecation banner in the outer spawn shell (same
  # reason as record_tmux_hotkey).
  local sid
  sid="$(virtui run --record --record-path "$cast" --cols "$COLS" --rows "$ROWS" -e BASH_SILENCE_DEPRECATION_WARNING=1 bash \
         | sed -n 's/^session_id: //p')"
  [[ -n "$sid" ]] || { warn "could not start session for $name"; return 1; }

  # The interactive story runs in a subshell so a hung/timed-out wait (Claude is
  # the flakiest surface here) degrades to a warning instead of aborting the whole
  # script — the cast written so far is kept, and cleanup below always runs.
  local rc=0
  (
    set -e
    # Switch to the REAL home + isolated log + throwaway project (invisible setup).
    virtui exec "$sid" "export HOME=\"$REAL_HOME\"; export FAILSAFE_LOG=\"$logfile\"; export PS1='\$ '; cd \"$projdir\"; clear" --wait-stable >/dev/null
    # tmux with the temp-dir config on a private socket.
    virtui exec "$sid" "tmux -L failsafe-claude-demo -f \"$conf\" new-session" --wait-stable >/dev/null
    virtui wait "$sid" --stable >/dev/null

    # Launch the real Claude Code CLI (real auth, user-global settings wire
    # `failsafe hook` as PreToolUse).
    type_line "$sid" "claude"
    virtui wait "$sid" --stable >/dev/null
    sleep 2
    # First-run "Do you trust the files in this folder?" dialog — accept if present.
    if virtui screenshot "$sid" | grep -qiE "trust"; then
      virtui press "$sid" Enter >/dev/null
      virtui wait "$sid" --stable >/dev/null
      sleep 2
    fi

    # Ask Claude to do the blocked thing in natural language. The deterministic
    # anchor is failsafe's reason ("… blocked while failsafe is enabled"), so we
    # wait on "blocked". If the operator's own config intercepts first (a skill,
    # a permission gate), the wait times out and this scenario degrades to a
    # warning — that interception is legitimate and must not be worked around.
    type_line "$sid" "delete the payments namespace with kubectl"
    virtui wait "$sid" --text "blocked" --timeout 180000 >/dev/null
    sleep "$STEP_PAUSE"

    # The toggle-without-leaving-Claude shot: the chord is consumed by tmux's root
    # key table (bind -n C-M-t) before Claude ever sees the bytes, so the guard
    # flips and the status bar reads 🔓 sudo around the untouched Claude TUI.
    virtui press "$sid" Escape Ctrl+T >/dev/null
    virtui wait "$sid" --text "sudo" >/dev/null
    sleep "$STEP_PAUSE"

    type_line "$sid" "/exit"                                 # leave Claude
    virtui wait "$sid" --stable >/dev/null
  ) || rc=$?
  [[ "$rc" -eq 0 ]] || warn "inside-Claude interaction did not complete cleanly (rc=$rc) — cast may be partial; other scenarios are unaffected."

  virtui exec "$sid" "exit" --wait-stable >/dev/null 2>&1 || true   # leave tmux (pane→server ends)
  virtui exec "$sid" "exit" --wait-stable >/dev/null 2>&1 || true   # leave outer bash
  virtui kill "$sid" >/dev/null 2>&1 || true
  tmux -L failsafe-claude-demo kill-server >/dev/null 2>&1 || true

  # Remove only the pane-mode file(s) our toggle created (never the user's live ones).
  ( cd "$panedir" && ls -1 ) > "$after" 2>/dev/null || true
  local newfile
  while IFS= read -r newfile; do
    if [[ -n "$newfile" ]]; then rm -f "$panedir/$newfile"; fi
  done < <(comm -13 "$before" "$after")
  rm -f "$before" "$after"
  # Proof the isolation held: the block that just rendered went to the throwaway
  # FAILSAFE_LOG, not the user's real decisions.jsonl.
  if [[ -s "$logfile" ]]; then
    log "inside-Claude demo: $(wc -l < "$logfile" | tr -d ' ') decision(s) captured in the isolated FAILSAFE_LOG (real decisions.jsonl untouched)."
  fi
  rm -rf "$projdir" "$cfgdir"
  rm -f "$logfile"
}

if [[ "$CLAUDE" == 1 ]]; then
  record_claude_hook || warn "inside-Claude demo failed; continuing with the other casts."
fi

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
