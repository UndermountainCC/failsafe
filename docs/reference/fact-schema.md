<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Fact Schema

The Rego `input` object passed to every policy rule — every field, its type, and its meaning.

Policies reference fields as `input.<field>`. All fields are always present (never undefined) except for enricher namespaces (`input.kubectl`, `input.git`), which are set only when the relevant enricher runs.

---

## Core fields

| Field | Type | Source | Description |
|-------|------|--------|-------------|
| `input.mode` | `string` | Mode chain | `"read"` or `"read & write"`. Bundled policies gate on `input.mode == "read"`. |
| `input.tool` | `string` | Tool registry | Registry name of the matched tool: `"kubectl"`, `"helm"`, `"terraform"`, `"aws"`, `"git"`, `"failsafe"`. |
| `input.verb` | `string` | Tool parser | First non-flag positional after the tool token. E.g. `"delete"` for `kubectl delete pods`. Empty string when no verb is present. |
| `input.subverb` | `string` | Tool parser | Second-level positional for compound commands. E.g. `"list"` for `terraform state list`, `"list"` for `helm repo list`. Empty string when not present. |
| `input.flags` | `object` | Tool parser | Parsed flags as a map. Short flags are expanded to their long names when a mapping exists. Boolean flags have value `true`; value-taking flags have a `string` value. E.g. `{"context": "arn:…", "dry-run": "server", "namespace": "payments"}`. |
| `input.positional` | `array<string>` | Tool parser | Remaining tokens after flags and the verb (and subverb). E.g. `["ns", "payments"]` for `kubectl delete ns payments`. |
| `input.env` | `object` | Shell parser | `KEY=VALUE` environment-variable prefixes on the command, e.g. `KUBECONFIG=... kubectl ...` → `{"KUBECONFIG": "..."}`. |
| `input.cwd` | `string` | Hook / `explain` | Effective working directory for this specific call. A `cd /path && CMD` prefix resolves to `/path`; otherwise the agent-reported `cwd`. |
| `input.now` | `string` | Clock | UTC timestamp in RFC3339 format at the moment `hook` was invoked. Useful for time-gated policies. |
| `input.raw` | `string` | Hook stdin | The full raw command string as the agent passed it, before parsing. Secrets are **not** redacted here — do not log `input.raw` from a policy. |
| `input.session` | `object` | Hook stdin | Session metadata — see table below. |

### `input.session` sub-fields

| Field | Type | Description |
|-------|------|-------------|
| `input.session.claude_session_id` | `string` | Agent session ID from the Claude Code hook envelope (`session_id` field). |
| `input.session.wezterm_pane` | `string` | `WEZTERM_PANE` environment variable at hook invocation time. Empty when not running under WezTerm. |

---

## Enricher namespaces

Enrichers populate per-tool sub-objects in `input`. Each enricher runs with a 100 ms deadline. If it panics, times out, or finds no data, the namespace is simply absent — policies that reference it will see undefined and the rule will not match.

### `input.kubectl`

Populated by the `kubectl_context` enricher, which runs for every `kubectl` command.

| Field | Type | Description |
|-------|------|-------------|
| `input.kubectl.current_context` | `string` | The raw `--context` flag value (verbatim). Always present when `--context` is given. |
| `input.kubectl.cluster_name` | `string` | Cluster name extracted from an EKS ARN. E.g. for `arn:aws:eks:us-east-1:123:cluster/prod`, this is `"prod"`. **Only present** when `--context` is an EKS ARN (`arn:aws:eks:…`, `arn:aws-us-gov:eks:…`, or `arn:aws-cn:eks:…`). Not populated for non-ARN context names. |

**Example — EKS ARN context:**

```rego
# Block kubectl mutations against a cluster whose name contains "prod".
block contains {"reason": "prod cluster is read-only"} if {
    input.tool == "kubectl"
    input.kubectl.cluster_name == "prod"
    not input.verb in {"get", "describe", "logs", "top"}
}
```

**Example — short context name:**

```rego
# Block against a non-ARN context named "production".
block contains {"reason": "production context is read-only"} if {
    input.tool == "kubectl"
    input.kubectl.current_context == "production"
    not input.verb in {"get", "describe", "logs"}
}
```

### `input.git`

Populated by the `git` enricher, which runs for every `git` command. The enricher reads `.git/config` and `.git/HEAD` by walking up from `input.cwd` — no subprocess, no network.

| Field | Type | Description |
|-------|------|-------------|
| `input.git.remote_url` | `string` | The `url` value from the `[remote "origin"]` section of `.git/config`. Absent when there is no `origin` remote. |
| `input.git.branch` | `string` | The current branch name from `.git/HEAD` (`ref: refs/heads/<branch>`). Absent when HEAD is detached. |

**Example:**

```rego
# Block force-push to main on the company monorepo.
block contains {"reason": "force-push to main is blocked"} if {
    input.tool == "git"
    input.verb == "push"
    input.flags["force"] == true
    input.git.branch == "main"
}
```

---

## Notes on flag representation

- **Boolean flags** (e.g. `--force`, `--dry-run` when no value follows) → value is `true` (boolean).
- **Value-taking flags** (e.g. `--context prod`, `--dry-run=server`) → value is a `string`.
- **Short flags** are expanded to their long equivalents when a mapping exists in the parser. E.g. kubectl's `-n` becomes `flags["namespace"]`.
- **Unknown long flags** are stored as `true` without consuming the next token (conservative: avoids swallowing a positional argument as a flag value).

The `--dry-run` flag for `kubectl` is a **string** flag (values: `"client"`, `"server"`, `"none"`, `"true"`). The bundled `kubectl.rego` carve-out checks:

```rego
input.flags["dry-run"] in {"client", "server", "true"}
```

---

## `test` corpus format

When using `failsafe test <path>`, each `fact.json` is the raw `input` map serialised as JSON. Enricher namespaces can be included directly:

```json
{
  "mode": "read",
  "tool": "kubectl",
  "verb": "delete",
  "subverb": "",
  "flags": { "context": "arn:aws:eks:us-east-1:123456789:cluster/prod" },
  "positional": ["ns", "payments"],
  "env": {},
  "cwd": "/home/user/project",
  "now": "2026-01-01T00:00:00Z",
  "raw": "kubectl --context arn:… delete ns payments",
  "session": { "claude_session_id": "", "wezterm_pane": "" },
  "kubectl": { "current_context": "arn:aws:eks:us-east-1:123456789:cluster/prod", "cluster_name": "prod" }
}
```
