<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Why failsafe

Your AI coding agent has your cloud credentials and a shell. The problem is that it is capable and fast, not that it is malicious. It will run `kubectl delete ns payments` if that is the next logical step in the task you gave it, and it will do it before you have finished reading the previous line of output. **failsafe is the guard that doesn't get tired.**

This page explains the problem that failsafe addresses, why existing approaches fall short, and what makes comprehension (rather than matching or sandboxing) the right level of abstraction for a command guard.

---

## Two failure modes, one root cause

When an AI coding agent goes wrong in ways that touch your infrastructure, it tends to go wrong in one of two ways:

1. **Careless mutation.** The agent is doing the right thing but picks the wrong target: applies to prod instead of staging, deletes the wrong namespace, destroys a live environment while cleaning up stale ones.
2. **Hijacked reasoning.** The agent's context has been poisoned, by a malicious `README`, a crafted error message, or a prompt-injection payload in a tool response, and it executes an attacker-chosen command under your credentials.

In both cases the question is the same: *should this specific command run right now?* The answer depends on understanding what the command does, not just what characters are in it.

---

## What keyword matching gets wrong

The simplest approach is string matching: block commands that contain `delete`, `destroy`, or `terraform apply`. Shell history is full of good reasons this fails.

A glob like `*delete*` blocks `kubectl delete pod/crashed-worker-7f9b2 --namespace dev` (a safe, targeted cleanup) and misses `kubectl patch deployment/payments -p '{"spec":{"replicas":0}}'`, which quietly scales your payment service to zero. The word "delete" is not the predicate. The combination of *tool*, *verb*, *target cluster*, and *current mode* is the predicate.

Glob matching also loses context that lives in flags rather than positional arguments. `kubectl apply --dry-run=server` is safe. `kubectl apply` against a prod cluster is not. A matcher that looks only at the command name and the first word after it cannot tell them apart.

---

## What OS sandboxing gets wrong

OS-level sandboxes (`seccomp`, `Landlock`, macOS sandbox profiles) operate on system calls or file paths. They can prevent a process from opening `/etc/shadow`, but they cannot prevent it from calling the AWS API to terminate an EC2 instance. The blast radius of a cloud-credentialed agent lives almost entirely in network calls, not local file writes.

Sandboxing is also binary: the process either has permission to make the call or it does not. The right answer is often contextual. `aws s3 ls` is fine in any mode; `aws s3 rm --recursive s3://prod-backups/` is not. A sandbox cannot express that distinction without essentially reimplementing a policy engine on top of itself.

---

## Comprehension as the differentiator

failsafe takes a different approach: it parses each command into a **structured fact** before any policy evaluation happens.

For a command like:

```console
kubectl --context arn:aws:eks:us-east-1:…:cluster/prod delete ns payments
```

the shell parser extracts the effective call through wrappers (`env`, `nice`, `command`, `bash -c`, and others). The tool registry maps the binary name to its parser. The kubectl parser walks the flag and argument structure, resolves `--context` against the local kubeconfig, and produces a fact like:

```json
{
  "tool": "kubectl",
  "verb": "delete",
  "resource": "ns",
  "kubectl": { "cluster_name": "prod", "context": "arn:aws:eks:us-east-1:…:cluster/prod" }
}
```

That fact is what the policy engine sees. A Rego rule can inspect `input.kubectl.cluster_name == "prod"` directly: no string parsing, no glob fragility. The policy expresses intent, and so does the fact.

This matters for coverage too. `kubectl delete` spelled as `/usr/local/bin/kubectl delete`, `KUBECONFIG=/tmp/k delete` via `kubectl`, `echo kubectl | xargs kubectl`, or nested inside `bash -c "..."` all produce the same fact after the shell parser and wrapper-peeling layers resolve them. The policy sees the same input regardless of how the agent chose to invoke the command.

---

## Fail closed on ambiguity

One consequence of the comprehension approach is that there are commands failsafe cannot safely model: subshells, `eval`, `source`, dynamic command names from variable expansion, control-flow constructs. A keyword matcher would let these through and hope for the best. failsafe refuses them and the engine treats refusal as a block.

This is a deliberate design choice. An agent can be told to rewrite the command in a statically-analyzable form. Silently allowing an unanalyzable command in the name of convenience is the opposite of what a guard should do.

See [Design Principles](./design-principles.md) for the full fail-closed discussion.

---

## Read-only by default, per pane

Beyond comprehension, failsafe establishes a mode boundary. Every terminal pane starts in `read` mode: the bundled policies block all mutating verbs across `kubectl`, `helm`, `terraform`, `aws`, and `git`. You flip a pane to `read & write` when you intend to make changes, and it flips back when you close it or reset it.

The per-pane granularity matters: you can run an agent in one pane (armored, read-only) while keeping another agent shell in another pane fully writable. The guard does not get in your way. It gets in the agent's way when the agent is about to do something you have not consciously allowed.

---

## Where to go next

- [Policy Cascade](./policy-cascade.md): how the three policy layers combine to reach a decision
- [Trust Model](./trust-model.md): why repo policies need explicit trust before they can loosen rules
- [Design Principles](./design-principles.md): fail-closed, local-first, and the audit log
- [How to write a policy](../how-to/per-cluster-policy.md): practical guide to adding your own Rego rules
