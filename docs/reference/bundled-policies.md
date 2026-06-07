<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Bundled Policies

The exact verbs and subverbs allowed or blocked by each bundled policy while failsafe is enabled.

Bundled policies live in `internal/embed/policies/` and are compiled into the binary. They are always the first layer in the chain (evaluated before user and repo policies). When failsafe is `disabled`, no bundled `block` rule fires. Mode is checked as the first condition in every rule via `input.failsafe_enabled`.

Policy packages follow the naming convention `failsafe.bundled.<tool>` (legacy `guard.bundled.<tool>` still accepted).

---

## `kubectl`: `failsafe.bundled.kubectl`

Source: `internal/embed/policies/kubectl.rego`

**Read-mode allowed verbs** (the `read_verbs` set):

```
get  describe  logs  exec  port-forward  top
version  cluster-info  config  explain
api-resources  api-versions  auth  diff  wait
```

**Special carve-out for `apply --dry-run`:** `kubectl apply` with `--dry-run=client`, `--dry-run=server`, or `--dry-run=true` is allowed even while failsafe is enabled.

**All other verbs are blocked** with reason: `"kubectl <verb> blocked while failsafe is enabled"`.

Examples of blocked verbs: `delete`, `scale`, `patch`, `drain`, `cordon`, `uncordon`, `taint`, `create`, `replace`, `rollout`, `label`, `annotate`, `expose`, `set`, `run`, `cp`, `attach`.

**Full rule (exact source):**

```rego
read_verbs := {
    "get", "describe", "logs", "exec", "port-forward", "top",
    "version", "cluster-info", "config", "explain",
    "api-resources", "api-versions", "auth", "diff", "wait",
}

block contains {"reason": sprintf("kubectl %s blocked while failsafe is enabled", [input.verb])} if {
    not input.failsafe_enabled == false
    input.tool == "kubectl"
    input.verb != ""
    not input.verb in read_verbs
    not allowed_dry_run
}

allowed_dry_run if {
    input.verb == "apply"
    input.flags["dry-run"] in {"client", "server", "true"}
}
```

---

## `helm`: `failsafe.bundled.helm`

Source: `internal/embed/policies/helm.rego`

**Read-mode allowed verbs** (the `read_verbs` set):

```
list  get  status  show  search  version  history  template
```

**`repo` verb, special subverb handling:** `helm repo list` is allowed; any other `helm repo <subverb>` (e.g. `add`, `remove`, `update`) is blocked with reason `"helm repo <subverb> blocked while failsafe is enabled"`.

**All other verbs are blocked** with reason `"helm <verb> blocked while failsafe is enabled"`.

Examples of blocked verbs: `install`, `upgrade`, `uninstall`, `rollback`, `push`, `package`, `dependency`.

**Full rule (exact source):**

```rego
read_verbs := {"list", "get", "status", "show", "search", "version", "history", "template"}

block contains {"reason": sprintf("helm %s blocked while failsafe is enabled", [input.verb])} if {
    not input.failsafe_enabled == false
    input.tool == "helm"
    input.verb != ""
    input.verb != "repo"
    not input.verb in read_verbs
}

block contains {"reason": sprintf("helm repo %s blocked while failsafe is enabled", [input.subverb])} if {
    not input.failsafe_enabled == false
    input.tool == "helm"
    input.verb == "repo"
    input.subverb != "list"
}
```

---

## `terraform` / `tofu`: `failsafe.bundled.terraform`

Source: `internal/embed/policies/terraform.rego`

Both `terraform` and `tofu` binaries are matched by the same tool parser and evaluated against this policy (`input.tool == "terraform"` for both).

**Read-mode allowed verbs** (the `read_verbs` set):

```
plan  show  output  validate  fmt  providers  version  graph
```

**`state` verb, special subverb handling:** `terraform state list` and `terraform state show` are allowed; any other subverb (e.g. `mv`, `rm`, `pull`, `push`) is blocked with reason `"terraform state <subverb> blocked while failsafe is enabled"`.

**All other verbs are blocked** with reason `"terraform <verb> blocked while failsafe is enabled"`.

Examples of blocked verbs: `apply`, `destroy`, `import`, `init`, `refresh`, `taint`, `untaint`, `workspace` (non-read subverbs).

**Full rule (exact source):**

```rego
read_verbs := {"plan", "show", "output", "validate", "fmt", "providers", "version", "graph"}

block contains {"reason": sprintf("terraform %s blocked while failsafe is enabled", [input.verb])} if {
    not input.failsafe_enabled == false
    input.tool == "terraform"
    input.verb != ""
    input.verb != "state"
    not input.verb in read_verbs
}

block contains {"reason": sprintf("terraform state %s blocked while failsafe is enabled", [input.subverb])} if {
    not input.failsafe_enabled == false
    input.tool == "terraform"
    input.verb == "state"
    not input.subverb in {"list", "show"}
}
```

---

## `aws`: `failsafe.bundled.aws`

Source: `internal/embed/policies/aws.rego`

The AWS CLI uses a `<service> <operation>` pattern, parsed as `verb` (service) + `subverb` (operation).

**Always allowed (no block rule matches):**

- Any `aws sts` command (all subverbs).
- `aws s3 ls`.
- Any operation whose name starts with `describe-`, `list-`, or `get-` on any service.
- A bare service invocation with no operation (e.g. `aws ec2`, `aws --help`).

**Blocked while failsafe is enabled:**

- `aws s3 <subverb>` where `subverb` is not `ls` and not empty. Reason: `"aws s3 <subverb> blocked while failsafe is enabled"`.
- Any `aws <service> <operation>` where service is not `sts` or `s3`, operation is not empty, and operation does not start with `describe-`, `list-`, or `get-`. Reason: `"aws <service> <operation> blocked while failsafe is enabled"`.

Examples of blocked operations: `aws s3 rm`, `aws ec2 run-instances`, `aws ec2 terminate-instances`, `aws eks create-cluster`, `aws eks delete-cluster`, `aws iam create-role`, `aws ecr batch-delete-image`.

**Full rule (exact source):**

```rego
block contains {"reason": sprintf("aws s3 %s blocked while failsafe is enabled", [input.subverb])} if {
    not input.failsafe_enabled == false
    input.tool == "aws"
    input.verb == "s3"
    input.subverb != ""
    input.subverb != "ls"
}

block contains {"reason": sprintf("aws %s %s blocked while failsafe is enabled", [input.verb, input.subverb])} if {
    not input.failsafe_enabled == false
    input.tool == "aws"
    input.verb != ""
    input.verb != "sts"
    input.verb != "s3"
    input.subverb != ""
    not is_read_action(input.subverb)
}

is_read_action(action) if startswith(action, "describe-")
is_read_action(action) if startswith(action, "list-")
is_read_action(action) if startswith(action, "get-")
```

---

## `git`: `failsafe.bundled.git`

Source: `internal/embed/policies/git.rego`

The bundled git policy is **intentionally empty**: it declares no `block` rules. Git is allowed by default. Individual repos enforce their own constraints (force-push guards, branch protections) via `.failsafe.rego`.

The file exists so that `failsafe policies list` includes `git` in the bundled set, making the coverage explicit.

---

## `failsafe`: `failsafe.bundled.failsafe`

Source: `internal/embed/policies/failsafe.rego`

This policy guards the `failsafe` binary itself (dogfood). Rules fire **regardless of mode**: the `enabled`/`disabled` distinction does not apply.

| Verb | Decision | Reason |
|------|----------|--------|
| `toggle` | BLOCK | `"failsafe toggle is user-only: toggle from your terminal (hotkey or failsafe toggle typed yourself), never via Claude"` |
| `hook` | BLOCK | `"failsafe hook is a lifecycle subprocess started by Claude Code; not for direct tool invocation"` |
| `mcp` | BLOCK | `"failsafe mcp is a lifecycle subprocess started by Claude Code; not for direct tool invocation"` |

All other `failsafe` subcommands (`explain`, `report`, `audit`, `trust`, `validate`, `tools`, `policies`, `mode`, `test`) are not blocked by this policy.

**Full rule (exact source):**

```rego
block contains {"reason": "failsafe toggle is user-only — toggle from your terminal (hotkey or `failsafe toggle` typed yourself), never via Claude"} if {
    input.tool == "failsafe"
    input.verb == "toggle"
}

block contains {"reason": sprintf("failsafe %s is a lifecycle subprocess started by Claude Code; not for direct tool invocation", [input.verb])} if {
    input.tool == "failsafe"
    input.verb in {"hook", "mcp"}
}
```
