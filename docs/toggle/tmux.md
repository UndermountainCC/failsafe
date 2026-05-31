<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# tmux: per-pane mode toggle

failsafe resolves the current pane's mode from `~/.claude/pane-mode/$TMUX_PANE` (see the
mode-source chain in the README). tmux's `#{pane_id}` format **equals** that `$TMUX_PANE`
env var, so a key binding can write the exact file the guard reads — instant, no chain
ambiguity. Missing file = `read` (the safe default).

Bind a key to flip the focused pane between `read` and `read & write`.

## 1. The toggle helper

Save as `~/.config/failsafe/tmux-toggle.sh` and `chmod +x` it. Writing the file directly
by pane id is the robust path — it sidesteps env edge cases when tmux runs *inside*
WezTerm/iTerm (where the chain might otherwise prefer a different pane var).

```bash
#!/usr/bin/env bash
# failsafe per-pane mode toggle for tmux. Arg: the tmux pane id (== $TMUX_PANE).
set -euo pipefail
pane="${1:?usage: tmux-toggle.sh <pane_id>}"
dir="$HOME/.claude/pane-mode"
file="$dir/$pane"
mkdir -p "$dir"

current="$(cat "$file" 2>/dev/null || echo read)"
[ "$current" = "read & write" ] && next="read" || next="read & write"
printf '%s' "$next" > "$file"          # canonical value the Rego policies match

tmux display-message "🔒 failsafe: $current → $next"
```

## 2. The key binding

In `~/.tmux.conf` — `Ctrl+Alt+T` with no prefix (mirrors the WezTerm binding):

```tmux
bind -n C-M-t run-shell "~/.config/failsafe/tmux-toggle.sh '#{pane_id}'"
```

Prefer a prefixed key? Use `bind T run-shell "…"` instead (then it's `prefix` + `T`).

Reload without restarting tmux:

```bash
tmux source-file ~/.tmux.conf
```

## 3. Status indicator

Show the focused pane's mode in the status bar. Save as
`~/.config/failsafe/tmux-status.sh` (`chmod +x`):

```bash
#!/usr/bin/env bash
mode="$(cat "$HOME/.claude/pane-mode/${1:-}" 2>/dev/null || echo read)"
if [ "$mode" = "read & write" ]; then
  printf '#[fg=yellow,bold]🔓 sudo#[default]'   # write enabled — make it loud
else
  printf '#[fg=green]🔒 read#[default]'
fi
```

Wire it into the status line in `~/.tmux.conf`:

```tmux
set -g status-interval 2
set -g status-right "#(~/.config/failsafe/tmux-status.sh '#{pane_id}') | %H:%M "
```

The `#[fg=…]` codes are interpreted by tmux, so a writable pane glows amber and reads
`🔓 sudo` — failsafe's `sudo` (see the WezTerm guide's
[*"sudo mode"*](wezterm.md#make-it-yours-sudo-mode) section for the full meme and the
auto-revert *sudo timeout* trick, which works here too: append
`( sleep 600; echo read > "$file" ) &` after the write in `tmux-toggle.sh`).

## Alternative: no helper script

If you'd rather not save a script, let `failsafe toggle` do it — but explicitly clear the
other pane vars so the mode-source chain can't pick the wrong one when tmux is nested
inside WezTerm/iTerm:

```tmux
bind -n C-M-t run-shell "WEZTERM_PANE= ITERM_SESSION_ID= TMUX_PANE='#{pane_id}' failsafe toggle"
```

(`failsafe` must be on `PATH` for tmux's `run-shell`.)

## Notes

- The file always stores the **canonical** value (`read` / `read & write`) — the same
  thing `failsafe mode set rw` / `ro` normalize to — so the toggle, the status bar, and
  the CLI all agree.
- `$TMUX_PANE` (e.g. `%5`) is unique per pane and stable for the pane's life, so each
  split keeps its own mode.
- Running tmux inside WezTerm or iTerm? The direct-write helper (step 1) is unaffected.
  The chain order is `WEZTERM_PANE` → `TMUX_PANE` → `ITERM_SESSION_ID`, which is why the
  no-script alternative clears the outer vars.
