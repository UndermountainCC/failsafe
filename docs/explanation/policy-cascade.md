<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Policy Cascade

failsafe evaluates every command against a chain of three policy layers. Each layer is a separate Rego file (or a directory of them). They are loaded in order — **bundled → user → repo** — and the engine combines their outputs into a single decision before replying to the agent.

This page explains what each layer is for, how decisions combine, and why only the repo layer is permitted to loosen rules that a higher layer has blocked.

---

## The three layers

### Bundled

The bundled policies are compiled into the failsafe binary. They express the conservative defaults that make sense for almost every team: block `kubectl delete`, `terraform destroy`, `helm uninstall`, `aws s3 rm`, force-push to a protected branch, and so on when the pane is in `read` mode.

You did not write these rules, and you cannot edit them without rebuilding the binary. That is intentional: they form a floor. An agent or a misconfigured config directory cannot remove them.

### User

The user layer lives at `~/.config/failsafe/policy.rego`. It is personal policy — additions or tightenings that reflect your own working practices regardless of which project you are in. For example, you might add a block rule that prevents any mutation against clusters whose names contain `prod` or `production`, regardless of what mode the pane is in.

The user layer can add new `block` rules. It cannot override a bundled block.

### Repo

The repo layer is a `.failsafe.rego` file discovered by walking up from the current working directory, stopping at your home directory. It is checked into the repository alongside the code it guards.

Because it travels with the repo, the repo layer is the right place for project-specific rules: allow `kubectl apply --dry-run=server` unconditionally (it is safe), block any mutation against a specific cluster name that is reserved for production, or tighten the rules for a sensitive service directory.

Crucially, the repo layer is the **only** layer that may declare `allow_override` rules — the mechanism for loosening a bundled or user block. This restriction is enforced at compile time: if a bundled or user module contains an `allow_override` rule head, the engine rejects it with an error rather than loading the policy.

---

## How decisions combine

Every layer evaluates independently against the same structured fact. The engine collects all `block` firings and all `allow_override` firings from all layers.

The outcome follows a single rule:

> **A command is blocked if any layer fires a block rule and no (trusted) repo layer fires an allow_override rule that cancels it.**

More precisely:

1. If any `block` rule fires with a malformed output (not a `{"reason": "..."}` object), the command is blocked immediately and the malformed reason is surfaced. A buggy block rule still blocks — it does not silently allow.
2. If any `allow_override` fires with a malformed output, that override is ignored. A buggy override does not free a real block.
3. If one or more well-formed `block` rules fire and no well-formed `allow_override` cancels them, the command is blocked. The reason surfaced is from the block rule closest to the current working directory (a repo-level block in the project root wins over the user layer, which wins over bundled).
4. If one or more `block` rules fire and at least one well-formed `allow_override` cancels them, the command is allowed, and the override reason is returned to the agent.
5. If no `block` rules fire, the command is allowed.

There is no "allow" rule to write. Absence of a block is allowance. The model stays simple: you describe what is forbidden, not what is permitted.

---

## Why only the repo layer can override

This design answers a specific question: *who should be able to loosen a bundled or user-level block?*

The bundled defaults exist because they represent conservative, generally-safe limits. A user policy adds personal limits on top. If either of those layers could also grant overrides, loosening a rule would happen in a file that is not visible in any particular project's version history. A teammate inherits your `~/.config/failsafe/policy.rego` without knowing it. A bundled policy could ship with an override that silently undoes itself.

Confining `allow_override` to the repo layer means that every loosening of a rule is:

- **Visible in the codebase.** The `.failsafe.rego` is a checked-in file. A code reviewer sees it. The git log records when it changed and who approved it.
- **Scoped to the project.** An override written for a dev cluster in repo A does not affect repo B. The override travels with the code it relaxes the guard for.
- **Subject to trust.** A repo's `allow_override` rules are silently stripped unless the repo has been explicitly trusted. The trust decision is yours to make, not the repo's. See [Trust Model](./trust-model.md).

The practical effect: when you see a `failsafe allowed (override)` decision in the audit log, you can always find the rule that caused it in the project's own `.failsafe.rego`. There is no other place to look.

---

## Reason precedence

When multiple layers fire block rules simultaneously, the engine surfaces the reason from the closest rule to the current working directory. A `.failsafe.rego` at the project root is closer than one two directories up, which is closer than the user layer, which is closer than bundled. This makes the displayed reason as specific and actionable as possible — you see the project rule that fired, not a generic bundled reason that also fired.

The `failsafe explain <cmd>` command shows the full trace — every rule that fired, its layer, file, and line number — so you can see the complete picture when the primary reason is not enough.

---

## Where to go next

- [Trust Model](./trust-model.md) — what happens to a repo's `allow_override` rules before trust is granted
- [Why failsafe](./why-failsafe.md) — the comprehension-first approach that makes structured facts possible
- [How to write a policy](../how-to/per-cluster-policy.md) — practical guide to writing your own block and override rules
