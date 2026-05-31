#!/usr/bin/env bash
# Copyright 2026 Undermountain Coding Company
# SPDX-License-Identifier: Apache-2.0

# failsafe ▸ Claude Code status line helper.
#
# Shows the current guard mode (and cwd / model) at the bottom of Claude Code, so
# you always know whether the agent can mutate infra — 🔒 read or 🔓 write.
#
# Wire it in ~/.claude/settings.json:
#   {
#     "statusLine": {
#       "type": "command",
#       "command": "~/.config/failsafe/claude-statusline.sh"
#     }
#   }
#
# Claude Code pipes a session JSON object on stdin (session_id, cwd, model, …) and
# renders the FIRST line of stdout as the status line. failsafe resolves the active
# pane's mode from that same environment, so the line tracks your `failsafe toggle`.
set -euo pipefail

input="$(cat)"

# Effective guard mode for the current pane/session (mode-source chain in the README).
mode="$(failsafe mode get 2>/dev/null | cut -f1)"
case "$mode" in
  "read & write") guard="🔓 write" ;;
  *)              guard="🔒 read"  ;;
esac

# cwd + model are nice context when jq is present; degrade gracefully without it.
extra=""
if command -v jq >/dev/null 2>&1; then
  dir="$(printf '%s' "$input" | jq -r '.cwd // empty' | sed "s|^$HOME|~|")"
  model="$(printf '%s' "$input" | jq -r '.model.display_name // empty')"
  [ -n "$dir" ]   && extra="$extra · $dir"
  [ -n "$model" ] && extra="$extra · $model"
fi

printf 'failsafe %s%s' "$guard" "$extra"
