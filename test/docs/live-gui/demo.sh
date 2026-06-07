#!/usr/bin/env bash
# Narrated failsafe demo — run in your WezTerm test window while screen-recording
# (with KeyCastr showing keypresses). It explains each step so a viewer understands
# WHAT is happening and WHY, and pauses for you to press the keybinding.
#
#   bash test/docs/live-gui/demo.sh
set -euo pipefail

say()  { printf '\n\033[1;36m▌ %s\033[0m\n' "$1"; }                 # cyan banner
note() { printf '  \033[2m%s\033[0m\n' "$1"; }                      # dim caption
key()  { printf '\n  \033[1;33m⌨  Press %s now (watch the tab badge), then Enter\033[0m ' "$1"; read -r; }
pause(){ printf '  \033[2m(Enter to continue)\033[0m'; read -r; }

# Friendly status, derived from the real `failsafe mode get`.
status() {
  local m; m="$(failsafe mode get | cut -f1)"
  if [ "$m" = "disabled" ]; then
    printf '  \033[1;33m🔓 failsafe DISABLED — the agent can write\033[0m  \033[2m(%s)\033[0m\n' "$m"
  else
    printf '  \033[1;32m🔒 failsafe ENABLED — writes blocked\033[0m  \033[2m(%s)\033[0m\n' "$m"
  fi
}

clear
say "failsafe — the fail-safe for AI coding agents"
note "While failsafe is ENABLED, the agent can read but every write or mutate"
note "(kubectl apply, terraform apply, rm…) is BLOCKED."
note "Need it to act? DISABLE failsafe for a moment — temporary and deliberate, like sudo."
pause

say "1 ·  This pane is protected"
status
note "failsafe is ENABLED. The tab badge shows  r."
pause

say "2 ·  Disable failsafe to let the agent write"
key "Ctrl+Alt+T"
status
note "failsafe is now DISABLED — writes allowed, on purpose. Badge turns amber  rw."
pause

say "3 ·  Re-enable failsafe"
key "Ctrl+Alt+T"
status
note "ENABLED again. (No file = ENABLED by default — fail-safe, not fail-open.)"
pause

say "Per-pane"
note "Each split or tab keeps its own state, so disabling failsafe in one pane"
note "never disables it in another."
echo
say "failsafe: protection is ON by default; turning it off is a visible, deliberate act."
