<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# failsafe

> **The fail-safe for AI coding agents.** Stop the command your agent can't take back — before it runs.

Your AI coding agent has your cloud credentials and a shell. **failsafe** sits in front of every command it runs and blocks the irreversible ones — `kubectl delete` on prod, `terraform destroy` — *before* they execute.

```console
$ kubectl --context arn:aws:eks:us-east-1:…:cluster/prod delete ns payments
⛔ failsafe  blocked  ·  kubectl delete against cluster=prod (read mode)
   reason: prod is read-only · policy: bundled/kubectl.rego:14
   flip this pane:  failsafe toggle
```

The only thing standing between a hijacked or careless agent and a wiped cluster is you, clicking "allow" on the hundredth prompt. **failsafe is the guard that doesn't get tired.**

---

## How it works

failsafe intercepts each shell command via a `PreToolUse` hook, parses it into a structured fact (tool, verb, flags, cluster context), and evaluates it against a cascade of OPA Rego policies. It returns `allow`, `deny`, or `ask` before execution — no network calls, no telemetry, every decision logged locally.

Each terminal pane carries its own **read/write mode**. By default, a pane is read-only: mutating verbs across `kubectl`, `helm`, `terraform`, and `aws` are blocked until you deliberately flip the pane to write. An agent pane stays armored while you keep a human pane writable.

---

## Where to go next

Not sure which section you need? Pick the one that matches what you want to do right now.

| I want to… | Go to |
| --- | --- |
| **Follow a step-by-step lesson** — install failsafe, wire the hook, and see a block happen | [Tutorial: Getting Started](tutorials/getting-started.md) |
| **Wire the Claude Code hook** in an existing project | [How-to: Claude Code hook](how-to/claude-code-hook.md) |
| **Block a specific cluster** with a custom Rego rule | [How-to: Per-cluster policy](how-to/per-cluster-policy.md) |
| **Look up a command or flag** | [Reference: CLI](reference/cli.md) |
| **Understand the design** — why Rego, why local-first, how the cascade works | [Explanation: Why failsafe](explanation/why-failsafe.md) |

!!! tip "New here?"
    Start with the [Tutorial: Getting Started](tutorials/getting-started.md). It takes about ten minutes and leaves you with a working guard.
