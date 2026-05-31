<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Claude Code status line: always-on guard mode

Claude Code can render a custom status line at the bottom of the session. Wire failsafe
into it and you always see whether the agent is in **🔒 read** (safe) or **🔓 write**
(it can mutate) — no guessing, no surprise `kubectl apply`.

The snippet: [`examples/claude-statusline.sh`](https://github.com/UndermountainCC/failsafe/blob/main/examples/claude-statusline.sh).

## How it works

Claude Code pipes a session JSON object on stdin (`session_id`, `cwd`, `model`, …) and
renders the **first line of stdout** as the status line. The helper asks failsafe for the
current pane's mode (`failsafe mode get`, resolved via the same mode-source chain the hook
uses) and prints, e.g.:

```
failsafe 🔒 read · ~/code/infra · Opus
failsafe 🔓 write · ~/code/infra · Opus
```

Because it reads the same per-pane mode the guard enforces, the line flips the instant you
`failsafe toggle` (or hit your `Ctrl+Alt+T` / `Ctrl+Opt+T` binding).

## Install

1. Copy the snippet somewhere stable and make it executable:

   ```bash
   mkdir -p ~/.config/failsafe
   cp examples/claude-statusline.sh ~/.config/failsafe/claude-statusline.sh
   chmod +x ~/.config/failsafe/claude-statusline.sh
   ```

2. Point Claude Code at it in `~/.claude/settings.json`:

   ```json
   {
     "statusLine": {
       "type": "command",
       "command": "~/.config/failsafe/claude-statusline.sh"
     }
   }
   ```

3. Start (or restart) Claude Code — the guard mode now lives in the status line.

## Notes

- `jq` is optional. With it you get cwd + model context; without it the line still shows
  the guard mode (the part that matters).
- The script calls `failsafe mode get`, so `failsafe` must be on `PATH` for the Claude
  Code process. If you install via the Homebrew cask that's automatic; for `go install`
  builds, ensure `$(go env GOPATH)/bin` is on `PATH`.
- Want the elevated state to shout? Pair it with the [toggle "sudo mode"](toggle/wezterm.md#make-it-yours-sudo-mode)
  styling so a writable pane is unmistakable in both the terminal *and* the status line.
