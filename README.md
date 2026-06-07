<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# 🛡️ failsafe

> **The fail-safe for AI coding agents.** Stop the command your agent can't take back — before it runs.

[![status](https://img.shields.io/badge/status-v0.1.0--dev-orange)](#-status--license)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue)](./LICENSE)
[![privacy](https://img.shields.io/badge/privacy-local--first%20·%20zero%20network-brightgreen)](#-why-failsafe)
[![docs](https://img.shields.io/badge/docs-undermountain.cc-blue)](https://undermountain.cc/labs/failsafe/)

📚 **Full documentation:** [undermountain.cc/labs/failsafe](https://undermountain.cc/labs/failsafe/)

Your AI coding agent has your cloud credentials and a shell. **failsafe** sits in front of
every command it runs and blocks the irreversible ones — `kubectl delete` on prod,
`terraform destroy` — *before* they execute.

```console
$ kubectl --context arn:aws:eks:us-east-1:…:cluster/prod delete ns payments
⛔ failsafe  blocked  ·  kubectl delete against cluster=prod (read mode)
   reason: prod is read-only · policy: bundled/kubectl.rego:14
   flip this pane:  failsafe toggle
```

The only thing between a hijacked or careless agent and a wiped cluster is you, clicking
"allow" on the hundredth prompt — and that fails exactly when you're tired or moving fast.
**failsafe is the guard that doesn't get tired.**

---

## 🚀 Why failsafe

- **🧠 Reads intent, not keywords.** A glob matcher sees an opaque `shell.exec("kubectl … delete …")` string; an OS sandbox blocks the legitimate command too. failsafe parses the *real* command into a structured fact — it knows `cluster=prod` from a dev cluster, and `kubectl apply` from `kubectl get`.
- **🔒 Read-only by default.** Destructive commands are blocked until you deliberately flip a pane to write. Per-pane: an agent pane stays armored while you keep a human pane writable.
- **🏠 Local-first.** Zero network calls, zero telemetry. Every policy check runs on your machine; every decision is logged locally.
- **🚫 Fails closed.** Can't safely parse a command, or a config is unreadable? It denies. That's the name.

---

## 🗺️ Supported surfaces

| Surface | Tools | Status |
| --- | --- | --- |
| ☸️ **Kubernetes** | `kubectl` (resolves `--context` → cluster), `helm` | ✅ Shipped |
| 🏗️ **Infra as Code** | `terraform`, `tofu` (`-chdir`, `state` subverbs) | ✅ Shipped |
| ☁️ **Cloud** | `aws` (service + operation aware) | ✅ Shipped |
| 🌿 **Git** | `git` (force-push, branch/remote aware) | ✅ Shipped |
| 📦 **JS/TS supply chain** | `npm`/`pnpm`/`yarn` install, postinstall, `npx` remote-exec | 🛠️ Roadmap |

---

## 📦 Install

**Homebrew** (after the first tagged release):

```bash
brew install undermountaincc/tap/failsafe
```

**From source:**

```bash
go install github.com/UndermountainCC/failsafe/cmd/failsafe@latest
```

The binary needs no configuration to start guarding against the bundled default policies.

---

## 🛠️ Quick start — Claude Code

Wire `failsafe` as a `PreToolUse` hook in `~/.claude/settings.json`:

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

Claude Code streams a JSON envelope on `stdin`; failsafe replies on `stdout` with `allow`,
`deny`, or `ask`. (The bare binary defaults to `hook`, so `"command": "failsafe"` also
works.) Agent-agnostic by design — anything with a pre-exec/Bash hook works.

---

## 🚥 Modes — per-pane read / write

Each terminal pane carries its own mode, so an agent can run read-only in one pane while
you keep another writable.

| Mode | | Behavior |
| --- | --- | --- |
| `enabled` | 🔒 | **Default.** Bundled policies block mutating verbs across `kubectl`, `helm`, `terraform`, `aws`. |
| `disabled` | 🔓 | Bundled blocks bypassed. User and repo policies still apply. |

```bash
failsafe toggle             # flip the current pane
failsafe mode get           # show the effective mode
failsafe mode set disabled  # disable failsafe   (aliases: off / rw / w / sudo)
failsafe mode set enabled   # re-enable failsafe (aliases: on / ro / r / safe)
```

**One-keystroke toggles, badges & status** — bind it in your terminal so you never type the command:

- 🪟 **WezTerm** — instant Lua toggle + tab badge: [`docs/toggle/wezterm.md`](docs/toggle/wezterm.md)
- 🍎 **iTerm2** — Python API toggle + key binding: [`docs/toggle/iterm.md`](docs/toggle/iterm.md)
- 🧱 **tmux** — key binding + status indicator: [`docs/toggle/tmux.md`](docs/toggle/tmux.md)
- 🤖 **Claude Code status line** — always-on guard mode: [`docs/claude-statusline.md`](docs/claude-statusline.md)

---

## ✍️ Policies (Rego)

Policies are [OPA Rego](https://www.openpolicyagent.org/docs/latest/) — the standard for
policy-as-code. They cascade across three layers: **bundled defaults** → **user**
(`~/.config/failsafe/`) → **repo** (`.failsafe.rego`).

```rego
package failsafe.user

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# Always allow `kubectl apply --dry-run=server`, regardless of mode.
allow_override contains {"reason": "dry-run is safe"} if {
    input.tool == "kubectl"
    input.verb == "apply"
    input.flags["dry-run"] == "server"
}

# Block any kubectl mutation against a prod cluster context.
block contains {"reason": "prod is read-only"} if {
    input.tool == "kubectl"
    input.kubectl.cluster_name == "prod"
    not input.verb in {"get", "describe", "logs", "top", "version"}
}
```

A command is **denied** if any layer `block`s it and no later layer `allow_override`s it.
Only the **repo** layer can override a bundled/user block — so loosened rules stay visible
in your codebase.

> ⚠️ **Repo trust.** `.failsafe.rego` files travel with a repo, so a cloned repo's rules
> are ignored until you trust them: `failsafe trust add /path/to/repo`. Untrusted repo
> policies are skipped with a warning; bundled + user layers still apply.

Drop-in templates (per-cluster, per-repo, custom tools) live in [`examples/`](examples/).

---

## 📊 Audit & reports

Every decision on a guarded tool is appended to `~/.config/failsafe/decisions.jsonl`.

- **🔑 Secrets redacted.** Token/password/credential flags and env assignments
  (`AWS_SECRET_ACCESS_KEY=…`) are masked to `***` before writing.
- **🔇 No noise.** Commands that touch no guarded tool (`ls`, `echo`, …) aren't logged.

```bash
failsafe report                           # last 7 days, Markdown summary
failsafe report --since 24h --format json # machine-readable
failsafe report --share                   # identity-scrubbed (paths, ARNs) — safe to paste
```

---

## 🧰 Command reference

| Command | What it does |
| --- | --- |
| `failsafe hook` | Read agent JSON on stdin, write a decision on stdout (default). |
| `failsafe toggle` | Flip the current pane's mode. |
| `failsafe mode get` / `set <rw\|ro>` | Read or set the pane's mode. |
| `failsafe explain <cmd>` | Dry-run a command through the policy chain; print which rules matched. |
| `failsafe report [--since][--format][--share]` | Summarize the decision log. |
| `failsafe audit [<path>]` | Print the effective policy chain (bundled/user/repo) with `file:line`. |
| `failsafe trust <add\|remove\|list>` | Manage trusted repo policies. |
| `failsafe validate [--strict] <file>` | Lint a `.rego` policy file. |
| `failsafe tools list` / `policies list` | List loaded parsers / active policies. |

---

## 🤝 Contributing

```bash
go test ./...                              # full suite
go vet ./...                               # static analysis
go run ./cmd/failsafe/ test test/corpus/   # policy regression corpus
```

- **Add a tool parser** — drop a YAML in `~/.config/failsafe/tools/` (see [`examples/tools/gh.yaml`](examples/tools/gh.yaml)); no recompile.
- **Add a policy** — improve the bundled defaults in `internal/embed/policies/`.
- **Add a regression** — a `fact.json` + `expected.json` pair under `test/corpus/`.

---

## 📄 Status & license

**Status:** `v0.1.0-dev` — pre-release. © 2026 Undermountain.

failsafe is dual-licensed:

- **Code** — all source (Go, build, CI) and the `examples/` policies/tools — is licensed
  under the [**Apache License 2.0**](./LICENSE).
- **Documentation** — the `docs/` site, this README, and other prose — is licensed under
  [**CC-BY-4.0**](./LICENSE-docs) (Creative Commons Attribution 4.0).

> failsafe is the command guard for AI coding agents — not the
> [`failsafe`](https://failsafe.dev) retry/resilience library.
