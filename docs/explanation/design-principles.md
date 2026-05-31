<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Design Principles

Three principles shape every decision in failsafe's implementation: it fails closed, it is local-first, and it keeps an honest record of every decision it makes. None of these is a marketing claim — each reflects a specific engineering constraint that follows from what failsafe is protecting.

---

## Fail closed

A security guard that says "I'm not sure, so I'll let it through" is not a security guard. failsafe's default on uncertainty is to block.

### Unanalyzable commands

The shell parser covers a broad range of real-world command patterns: pipelines, `&&`-chained commands, `env`/`nice`/`command` wrappers, `bash -c "..."`, and `xargs` with safe boolean flags. When it encounters something it cannot safely model — a subshell `(...)`, `eval`, `source`, a dynamic command name from variable expansion, a control-flow construct like `if` or `for` — it returns a refuse reason instead of a list of effective calls. The engine treats refusal as a block.

This means an agent cannot escape policy evaluation by wrapping a command in a layer of shell complexity. The correct response to a refusal is to rewrite the command in a statically-analyzable form.

### Unreadable config

If the user policy file at `~/.config/failsafe/policy.rego` exists but cannot be read — permission denied, I/O error — failsafe returns an error and blocks the command. It does not silently skip the policy as if it were absent.

The same applies to repo policies discovered by walking up the directory tree. A policy that was present yesterday and became unreadable today could mean anything: filesystem corruption, an overzealous `chmod`, or an attacker who knows the filename. Silently ignoring it would make the policy disappear without any indication that something changed.

The code makes this explicit:

```go
// Permission denied, EIO, etc. — fail closed: an existing
// policy that we can't load must not silently disappear.
return nil, fmt.Errorf("read user policy %s: %w", userPath, err)
```

A missing file (the common case where no policy has been written yet) is fine — `os.IsNotExist` is handled separately and treated as "no policy here." But an error on a file that exists is always fatal to the policy load.

### Malformed rule output

Rego rules can produce any output. failsafe requires block and allow_override rules to emit objects with a `"reason"` string field. If a rule fires but its output does not match that shape, failsafe does not silently ignore it:

- A malformed **block** rule still blocks. The reason surfaced is a policy-bug message rather than the rule's intended reason, but the command is not allowed through.
- A malformed **allow_override** rule does not unblock. A buggy override cannot free a real block.

The trace visible in `failsafe explain` always includes malformed hits so policy authors can find and fix them.

### The name

The name "failsafe" is literal. The mechanism that makes nuclear systems safe is the one that defaults to the safe state when power is lost or signals are ambiguous. In this system, the safe state is "blocked."

---

## Local-first

failsafe makes zero network calls. No policy is fetched from a remote server. No decision is reported to a cloud service. No telemetry is sent anywhere.

This is not a privacy feature bolted on after the fact — it is a prerequisite for the use case. A guard that requires a network call to evaluate a policy is unavailable the moment your laptop goes offline, your corporate proxy misbehaves, or the remote service has an outage. Any of those conditions would mean the guard either blocks everything (unworkable) or fails open (the opposite of what it should do).

The bundled policies are compiled into the binary. The user and repo policies are local files. OPA's Rego evaluator runs entirely in-process. Every decision is made on your machine, by your process, against your policies, in your shell session.

The command-line verbs that do involve external state (`failsafe trust add`, `failsafe mode set`) only modify files on your local filesystem — the trust list and the per-pane mode file.

---

## Audit log and secret redaction

Every decision on a guarded command is appended to `~/.config/failsafe/decisions.jsonl` as a JSON line. The log records what was decided, why, which tool and verb were involved, the current mode, and context about the agent session and terminal pane.

The log file is owner-readable only (`0o600`), created in an owner-only directory (`0o700`). These permissions are the backstop — the primary control is redaction.

### What gets redacted

Before any command string is written to the log, it passes through `DefaultRedact`. Three patterns are replaced with `***`:

- `--flag=VALUE` where the flag name contains a word like `token`, `secret`, `password`, `credential`, `api-key`, or `auth`
- `--flag VALUE` (space-separated) with the same flag name heuristic
- `KEY=VALUE` environment assignments where the key name contains the same secret-ish words

The patterns are intentionally broad. A false positive — redacting a non-secret value — produces a slightly less readable log entry. A false negative — missing a credential — produces a log entry that leaks a secret. The heuristic errs toward over-redaction.

```
# Example: token stays out of the log
kubectl --token=eyJhbGci... get pods
# logged as:
kubectl --token=*** get pods
```

### Logging must not block

The audit log is observability, not enforcement. If the log write fails — disk full, permissions changed, I/O error — the hook still returns its decision to the agent. The failure is returned for tests to inspect, but the hook is designed to ignore it. An infrastructure guard that stops working because its log file is full would be worse than no guard at all.

### Non-guarded commands are not logged

Commands that do not involve a guarded tool — `ls`, `echo`, `grep`, `go build`, `npm test` — produce no log entry. The log is not a shell history; it is a record of security-relevant decisions. Logging every command would bury the signal in noise and create a new privacy exposure.

---

## Where to go next

- [Why failsafe](./why-failsafe.md) — why comprehension is the right foundation for these principles
- [Policy Cascade](./policy-cascade.md) — how fail-closed behavior extends to the policy evaluation logic
- [Trust Model](./trust-model.md) — how the same fail-closed philosophy applies to repo policies
