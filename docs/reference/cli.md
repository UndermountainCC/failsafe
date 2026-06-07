<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# CLI Reference

Every subcommand shipped with the `failsafe` binary: synopsis, flags, what it reads/writes, and exit codes.

---

## Global flags

| Flag | Effect |
|------|--------|
| `--version`, `-v` | Print `failsafe <version>` and exit 0. |
| `--help`, `-h` | Print usage summary and exit 0. |

**Default behaviour:** when called with no subcommand and no flag arguments, `failsafe` runs `hook` mode (reads Claude Code JSON on stdin). This matches the Claude Code hook configuration `"command": "failsafe"`.

---

## `hook`

```
failsafe hook
failsafe          # implicit hook, same behaviour
```

The hot path. Reads one Claude Code `PreToolUse` JSON envelope from **stdin**, evaluates the `tool_input.command` field through the shell parser, tool registry, and OPA policy chain, then writes a decision to **stdout** and exits.

**Stdin format** (Claude Code `PreToolUse` JSON):

```json
{
  "tool_name": "Bash",
  "tool_input": { "command": "kubectl get pods" },
  "cwd": "/home/user/project",
  "session_id": "abc123"
}
```

**Stdout on block:**

```json
{"decision":"block","reason":"kubectl delete blocked while failsafe is enabled"}
```

**Stdout on allow (plain):** empty. Claude Code interprets exit 0 with no stdout as allow.

**Stdout on allow with repo override:**

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"🔓 allowed by repo policy: <reason>"}}
```

**What it reads/writes:**

- Reads: mode from the chain (`FAILSAFE_MODE`, pane-mode files; see [Modes](modes.md)).
- Reads: `~/.config/failsafe/policy.rego` (user layer), `.failsafe.rego` walking up from `cwd` (repo layer).
- Reads: `~/.config/failsafe/trusted-repos.yaml` (trust list).
- Appends: one JSON line to `~/.config/failsafe/decisions.jsonl` (or `FAILSAFE_LOG` override) per infra-tool decision. Logging is best-effort; a write failure never blocks the command.

**Fail-closed behaviour:**

- Shell parse error: block.
- Refuse-on-ambiguity (subshell, eval, uncertain `cd`, heredoc, etc.): block.
- Uncertain `cwd` before a registered tool call: block.
- Registry build error (corrupt YAML tool): block.
- Policy compile error: exit 1 (non-zero, hook aborted).

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Decision emitted (allow or block: both are exit 0; Claude Code reads stdout to distinguish). |
| 1 | Fatal internal error (policy compile failure, I/O error reading stdin). |

---

## `mcp`

```
failsafe mcp
```

Runs a stdio JSON-RPC 2.0 MCP server. Reads one request per line from **stdin**, writes one response per line to **stdout**. Runs until EOF; exits 0 on clean EOF, 1 on write error.

Exposes two MCP tools:

| Tool | Description |
|------|-------------|
| `check_mode` | Returns the current mode (`enabled` or `disabled`) and pane ID. No arguments. |
| `toggle_mode` | Flips the mode atomically. No arguments. Returns `{old, new, pane_id}`. |

Both tools use the same mode chain as `hook` (env → pane-mode files → TTY file → global file).

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Clean EOF. |
| 1 | Write error on stdout. |

---

## `toggle`

```
failsafe toggle
```

Flips the first writable mode source between `enabled` and `disabled`. Writes atomically (temp-file + rename).

Prints `<old> → <new> (<path>)` on success, e.g.:

```
enabled → disabled (/home/user/.claude/pane-mode/12345)
```

**What it reads/writes:** the first writable source in the mode chain whose path resolves in the current environment. See [Modes](modes.md) for the chain order.

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Mode flipped successfully. |
| 1 | No writable source found, or write error. |

---

## `mode get`

```
failsafe mode get
```

Prints the effective mode and its source, e.g.:

```
disabled    (file: /home/user/.claude/pane-mode/12345)
enabled     (default; no source resolved)
enabled     (env)
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Mode printed. |
| 1 | Resolution error. |

---

## `mode set`

```
failsafe mode set <value>
```

Sets the mode by writing to the first writable source. Accepts aliases:

| Alias | Canonical value written |
|-------|------------------------|
| `enabled`, `enable`, `on`, `closed`, `close`, `lock`, `ro`, `r`, `read`, `safe` | `enabled` |
| `disabled`, `disable`, `off`, `open`, `unlock`, `rw`, `w`, `write`, `sudo` | `disabled` |

Matching is case-insensitive.

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Mode written. |
| 1 | No writable source found, or write error. |
| 2 | Invalid value string. |

---

## `explain`

```
failsafe explain <command>
failsafe explain "kubectl --context arn:… delete ns payments"
```

Dry-runs a shell command through the same pipeline as `hook` and prints the per-call decision: which rules matched, from which layer, with what reason. Does **not** write to the audit log and does **not** flip any mode. Stops printing at the first block, mirroring `hook` behaviour.

Accepts the command either as a single quoted string or as shell-split tokens (the user's shell does the splitting).

**Output per extracted call:**

```
── call 1: kubectl ──
Verb:          delete
Positional:    ns payments
Flags:
  context         = arn:aws:eks:us-east-1:…:cluster/prod
Effective cwd: /home/user/project
Mode:          enabled
Policy chain (3 modules at this cwd):
  [bundled] bundled/kubectl.rego
  [user] /home/user/.config/failsafe/policy.rego
  [repo] /home/user/project/.failsafe.rego [trusted]
Decision: BLOCK
Reason  : kubectl delete blocked while failsafe is enabled
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Explanation printed (including a BLOCK decision, which is not a failure). |
| 2 | Shell parse error or internal error (registry build, policy compile). |

---

## `report`

```
failsafe report [--since <window>] [--format <fmt>] [--share]
```

Reads `decisions.jsonl`, filters to a time window, aggregates by tool/verb/decision, and prints a summary.

**Flags:**

| Flag | Default | Accepts |
|------|---------|---------|
| `--since` | `7d` | `<N>d` (days) or any Go duration (`24h`, `30m`, `1h30m`). |
| `--format` | `md` | `md` (Markdown) or `json`. |
| `--share` | off | When set, redacts deployment-identifying data (paths, ARNs) before rendering. |

**What it reads:** `~/.config/failsafe/decisions.jsonl`, or the path from `FAILSAFE_LOG` if set. A missing log file is not an error; a fresh install prints "No decisions logged in this window."

**Markdown output structure:**

1. Window header and total count.
2. Table: counts by tool / verb / decision.
3. "Scariest decisions" list: up to 5 entries scored by risk heuristic (block > allow_override; prod-keyword bumps the score).

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Report rendered. |
| 1 | Log file I/O error. |
| 2 | Invalid flag value. |

---

## `audit`

```
failsafe audit [<path>]
```

Prints the effective policy chain in force at `<path>` (defaults to `$PWD`). Lists every `block` rule count and every `allow_override` rule with `file:line` and its static reason string. Untrusted repo policies are included in the listing (their `allow_override` rules are flagged `[untrusted]` and skipped at eval time).

**Example output:**

```
Policy layers in effect at /home/user/project:

  [bundled]
    bundled/kubectl.rego — 1 block rule(s)
    bundled/helm.rego — 2 block rule(s)
    …

  [user]
    /home/user/.config/failsafe/policy.rego — 1 block rule(s)

  [repo]
    /home/user/project/.failsafe.rego — 0 block rule(s)
      ⚠  allow_override at /home/user/project/.failsafe.rego:12 — "dry-run is safe"

5 block rules total, 1 allow_override.
Run `failsafe explain <command>` to dry-run specific commands.
```

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Chain printed. |
| 2 | Policy discovery error. |

---

## `trust`

```
failsafe trust add <path>
failsafe trust add .        # find .failsafe.rego at or above $PWD
failsafe trust .            # same as above
failsafe trust remove <path>
failsafe trust list
failsafe trust check [<path>]
```

Manages the trusted-repos list at `~/.config/failsafe/trusted-repos.yaml`.

**Subcommands:**

| Subcommand | Effect |
|------------|--------|
| `add <path>` | Add `<path>` to the trust list. Idempotent (already-trusted is a clean exit 0). |
| `add .` / `.` | Walk up from `$PWD` to find the directory containing `.failsafe.rego`; trust that directory. |
| `remove <path>` | Remove from the trust list. Exit 1 if not present. |
| `list` | Print all trusted repo paths with their `added_at` timestamps and optional reasons. |
| `check [<path>]` | Exit 0 if `<path>` (or `$PWD`'s repo root) is trusted, exit 1 otherwise. |

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Operation succeeded (or already-trusted for `add`). |
| 1 | `remove`: path not in list; `check`: path not trusted; write error. |
| 2 | Usage error or I/O error reading the trust file. |

---

## `validate`

```
failsafe validate [--strict] <path>
```

Lints a `.rego` file for use as a failsafe policy. Checks in order:

1. **Parse**: valid Rego syntax.
2. **Package**: `package failsafe.repo` for `.failsafe.rego`, `package failsafe.user` for files under `~/.config/failsafe/`. (Legacy `guard.repo` / `guard.user` are still accepted — dual-namespace.)
3. **Rule names**: `allow_override` is reserved for `failsafe.repo`; bundled and user layers may only declare `block`.
4. **Rule shape**: every `block` and `allow_override` rule must produce `{"reason": <string>}`.
5. **Fact-field references**: `input.<X>` references are checked against the known field list; unknowns emit a warning.

`--strict` promotes warnings (fact-field unknowns) to errors.

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Valid (possibly with warnings when `--strict` is not set). |
| 1 | Policy file is invalid. |
| 2 | File not found, usage error, or I/O error. |

---

## `tools list`

```
failsafe tools list
```

Lists all registered tool parsers with their source. Each row: `<name>  (<source>)`.

Sources:

- `built-in Go`: Go-coded parsers (`kubectl`, `helm`).
- `bundled YAML`: YAML tool definitions embedded in the binary (`terraform`, `aws`, `git`).
- `user YAML at <path>`: YAML files from `~/.config/failsafe/tools/`.

**Exit codes:** 0 always.

---

## `policies list`

```
failsafe policies list
```

Lists all active policy modules. Bundled modules always shown first, then user and repo modules discovered from the current directory.

```
[bundled] aws.rego
[bundled] failsafe.rego
[bundled] git.rego
[bundled] helm.rego
[bundled] kubectl.rego
[bundled] terraform.rego
[user]    /home/user/.config/failsafe/policy.rego
[repo]    /home/user/project/.failsafe.rego
```

**Exit codes:** 0 always.

---

## `test`

```
failsafe test <path>
```

Runs a policy regression corpus. `<path>` may be:

- A directory: walks it for subdirectories each containing `fact.json` + `expected.json`.
- A single `fact.json` file: runs that one case.

Each `fact.json` is the raw Rego `input` object (see [Fact Schema](fact-schema.md)). Each `expected.json` declares:

```json
{
  "block": true,
  "reason_contains": "kubectl delete blocked",
  "override_reason_contains": ""
}
```

Prints `PASS`/`FAIL` per case and a summary count. Exit 1 if any case fails.

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | All cases passed. |
| 1 | One or more cases failed. |
| 2 | Path not found, file unreadable, or JSON parse error. |
