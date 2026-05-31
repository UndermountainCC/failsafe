<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Wire failsafe into Claude Code

Put failsafe in front of every Bash command Claude Code runs, so destructive infra commands are blocked before they execute.

## 1. Install the binary

```bash
brew install undermountaincc/tap/failsafe   # Homebrew (recommended)
# or, from source (needs Go 1.22+):
go install github.com/UndermountainCC/failsafe/cmd/failsafe@latest
```

Make sure `failsafe` is on the `PATH` of the shell Claude Code launches. The binary needs no configuration to start guarding against the bundled default policies.

## 2. Register the PreToolUse hook

Add a `PreToolUse` hook to `~/.claude/settings.json` that matches the `Bash` tool:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "failsafe hook" }]
      }
    ]
  }
}
```

Claude Code streams a JSON envelope on `stdin`; failsafe replies on `stdout` with `allow`, `deny`, or `ask`. The bare binary defaults to `hook`, so `"command": "failsafe"` also works.

!!! note "Agent-agnostic by design"
    Anything with a pre-exec or Bash hook works the same way. The hook reads the agent's command on stdin and writes a decision on stdout — there is nothing Claude Code-specific in the protocol.

## 3. Confirm it's live

In a fresh pane (read-only by default), ask Claude Code to run a mutating command, or dry-run it yourself:

```bash
failsafe explain "kubectl delete ns payments"
```

A blocked command prints a `BLOCK` decision with the matching reason. If it does, the chain is wired correctly.

## 4. Flip a pane to write when you mean to

failsafe stays read-only until you deliberately flip a pane:

```bash
failsafe toggle        # flip the current pane
```

Bind a one-keystroke toggle in your terminal so you never type the command — see [WezTerm](../toggle/wezterm.md), [iTerm2](../toggle/iterm.md), or [tmux](../toggle/tmux.md).

## See also

- [Modes — per-pane read / write](../reference/modes.md)
- [Why failsafe](../explanation/why-failsafe.md)
- [Explain a command](./explain-a-command.md)
