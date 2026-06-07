<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Protect a repo with a checked-in policy

Gate infra commands at the repository level by committing a `.failsafe.rego` to the repo root, so the rules travel with the code and apply to anyone (or any agent) working in that directory.

## 1. Create `.failsafe.rego`

In the root of the repository:

```bash
touch .failsafe.rego
```

The file must declare `package failsafe.repo`. This is the only layer that can write `allow_override` rules to loosen a bundled or user-level block.

Here is the policy from [`examples/policies/per-repo-protection.rego`](https://github.com/UndermountainCC/failsafe/blob/main/examples/policies/per-repo-protection.rego) as a starting point:

```rego
package failsafe.repo

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# This repo's manifests must be applied via CI, not local kubectl.
block contains {"reason": "this repo: kubectl apply via CI only — submit a PR"} if {
    input.tool == "kubectl"
    input.verb == "apply"
    not input.flags["dry-run"]
}

# No force-push to the org's default branches.
block contains {"reason": "no force-push to acme default branches"} if {
    input.tool == "git"
    input.verb == "push"
    "--force" in input.positional
    contains(input.git.remote_url, "acme/")
}
```

Adjust the `contains(input.git.remote_url, …)` predicate to match your own org slug.

!!! tip "Dry-run carve-out"
    `input.flags["dry-run"]` is `"server"` or `"client"` when the flag is present. The first block above permits `kubectl apply --dry-run=server` by checking `not input.flags["dry-run"]`. Remove that line if you want to block dry-runs too.

## 2. Validate the file

```bash
failsafe validate .failsafe.rego
```

The validator checks the package name, rule names, rule shapes (`{"reason": …}`), and known fact fields:

```
✓ parse OK
✓ package: failsafe.repo
✓ rule names: no reserved-rule violations
✓ rule shapes: all block/allow_override return {"reason": ...}
✓ fact-field references: all known

OK.
```

Run with `--strict` to fail on any `⚠` warnings (useful in CI).

## 3. Trust the repo

`.failsafe.rego` files travel with a repo, so they are not loaded until you explicitly trust the repo's path. From inside the repo:

```bash
failsafe trust add .
```

Or pass the absolute path:

```bash
failsafe trust add /path/to/your/repo
```

See [Trust a repo](./trust-a-repo.md) for the full workflow, including listing and removing entries.

!!! warning "Untrusted repos"
    A `.failsafe.rego` in an untrusted repo is silently skipped. Bundled and user policies still apply. You will see `[untrusted]` next to the repo file in `failsafe explain` output.

## 4. Dry-run a command through the full chain

With the repo trusted, test that the policy fires:

```bash
cd /path/to/your/repo
failsafe explain "kubectl apply -f manifests/"
```

Expected output (repo layer loaded and trusted):

```
── call 1: kubectl ──
Verb:        apply
Effective cwd: /path/to/your/repo
Mode:        read
Policy chain (2 modules at this cwd):
  [bundled] bundled/kubectl.rego
  [repo] /path/to/your/repo/.failsafe.rego [trusted]
Decision: BLOCK
Reason  : this repo: kubectl apply via CI only — submit a PR
```

## 5. Commit and review with teammates

`.failsafe.rego` is a first-class source file. Commit it, review it in PRs, and keep it next to the infra code it governs.

```bash
git add .failsafe.rego
git commit -m "chore: add failsafe repo policy"
```

Each team member who clones the repo runs `failsafe trust add .` once after cloning. Each agent pane needs the same trust entry to load the repo layer.

## See also

- [Trust a repo](./trust-a-repo.md)
- [Explain a command](./explain-a-command.md)
- [Write a per-cluster user policy](./per-cluster-policy.md)
- [Policy cascade](../explanation/policy-cascade.md)
- [Trust model](../explanation/trust-model.md)
