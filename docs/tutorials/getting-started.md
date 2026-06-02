<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Tutorial: Getting Started

In this tutorial you will install failsafe, wire it as a Claude Code hook, and watch it block a destructive command, all without a live agent or a real cluster. By the end you will have a working guard and a clear mental model of how the read/write mode switch works.

**Time:** about 10 minutes.  
**Prerequisites:** Claude Code installed and a terminal (plus Go 1.22+ only if you install from source).

---

## Step 1: Install failsafe

**Homebrew (recommended):**

```bash
brew install undermountaincc/tap/failsafe
```

**Or from source** (needs Go 1.22+):

```bash
go install github.com/UndermountainCC/failsafe/cmd/failsafe@latest
```

Verify it is on your `PATH`:

```bash
failsafe --version
```

```console
failsafe 0.1.0
```

The binary ships with bundled policies. No configuration is required to start guarding.

---

## Step 2: Wire the Claude Code hook

Open (or create) `~/.claude/settings.json` and add a `PreToolUse` hook that hands every `Bash` tool call to failsafe:

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

Claude Code streams a JSON envelope on `stdin`; failsafe replies on `stdout` with `allow`, `deny`, or `ask`. The bare `failsafe` binary defaults to the `hook` subcommand, so `"command": "failsafe"` also works.

!!! tip "Already have a hooks section?"
    Append the new entry to the existing `PreToolUse` array; don't replace it.

Save the file. The hook takes effect immediately for new Claude Code sessions; no restart needed.

---

## Step 3: See the block in action

You don't need a live agent or a real cluster to see failsafe work. The `explain` subcommand runs any command through the full policy chain and prints the decision.

Run the same command from the README:

```bash
failsafe explain "kubectl --context arn:aws:eks:us-east-1:123456789012:cluster/prod delete ns payments"
```

You should see:

```console
── call 1: kubectl ──
Verb:        delete
Positional:  ns payments
Effective cwd: /your/current/dir
Mode:        read
Policy chain (1 modules at this cwd):
  [bundled] kubectl.rego
Decision: BLOCK
Reason  : kubectl delete blocked in read mode
```

`explain` always evaluates in **read** mode, the safe default. The bundled `kubectl.rego` policy blocks every non-read verb (`delete`, `apply`, `scale`, …) when mode is `read`. `kubectl get`, `kubectl describe`, and `kubectl logs` would be allowed.

!!! success "You just verified the guard"
    failsafe parsed the command, identified the tool (`kubectl`), extracted the verb (`delete`), and blocked it before any agent or cluster was involved.

---

## Step 4: Flip to write mode and back

When you *do* want to allow mutations, for example while you are actively working in a cluster you trust, flip the current pane to write:

```bash
failsafe mode set rw
```

Confirm the mode changed:

```bash
failsafe mode get
```

```console
read & write
```

Now run a read-only `kubectl get` to confirm allowed commands still pass through:

```bash
failsafe explain "kubectl get pods -n payments"
```

```console
── call 1: kubectl ──
Verb:        get
Positional:  pods
Effective cwd: /your/current/dir
Mode:        read
Policy chain (1 modules at this cwd):
  [bundled] kubectl.rego
Decision: ALLOW
```

!!! note
    `explain` always evaluates in read mode regardless of your pane's current mode: it is a safe dry-run tool. To observe the effect of write mode on a live agent session, set `rw` and then run Claude Code in that pane; `failsafe hook` (not `explain`) reads the pane's actual mode file.

When you are done, lock the pane back to read-only:

```bash
failsafe mode set ro
```

Or use the shorthand toggle (useful for a terminal keybinding):

```bash
failsafe toggle
```

`failsafe mode get` will confirm you are back to `read`.

!!! tip "Per-pane, not per-shell"
    Mode is stored per pane (keyed by `$WEZTERM_PANE`, `$TMUX_PANE`, or `$ITERM_SESSION_ID`). You can keep an agent pane locked to `read` while a human pane next to it is in `read & write`. See the toggle docs for one-keystroke bindings in [WezTerm](../toggle/wezterm.md), [iTerm2](../toggle/iterm.md), and [tmux](../toggle/tmux.md).

---

## What you built

- failsafe is installed and on your `PATH`.
- Claude Code will call `failsafe hook` before every `Bash` command an agent runs.
- The bundled policies block destructive `kubectl`, `helm`, `terraform`, and `aws` commands while your pane is in read mode (the default).
- You know how to flip a pane to write, verify the mode, and flip it back.

---

## What's next

Now that you have a working guard, explore the rest of the docs:

- **Do a specific thing**: [How-to: Wire the Claude Code hook](../how-to/claude-code-hook.md) goes deeper on hook configuration options; [How-to: Per-cluster policy](../how-to/per-cluster-policy.md) shows how to write a Rego rule that blocks your prod cluster by name.
- **Understand the design**: [Explanation: Why failsafe](../explanation/why-failsafe.md) covers why structured parsing beats glob matching, why local-first matters, and how the bundled → user → repo policy cascade works.
- **Look up a command**: [Reference: CLI](../reference/cli.md) lists every subcommand, flag, and exit code.
