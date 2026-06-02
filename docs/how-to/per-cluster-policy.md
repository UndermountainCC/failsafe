<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Write a per-cluster policy

Make a specific Kubernetes cluster read-only, regardless of the pane's mode, by adding a user-level Rego rule that names the cluster explicitly.

## 1. Locate your cluster name

failsafe resolves the cluster name from the `--context` flag or your current `kubectl` context. Find the canonical name:

```console
$ kubectl config get-contexts
CURRENT   NAME                    CLUSTER    AUTHINFO   NAMESPACE
*         arn:aws:eks:…/prod      prod       …          default
          arn:aws:eks:…/staging   staging    …          default
```

The value in the **NAME** column becomes `input.kubectl.cluster_name` inside your policy. (If the name is a long ARN, use the exact ARN string.)

## 2. Create the user policy file

```bash
mkdir -p ~/.config/failsafe
```

Open `~/.config/failsafe/policy.rego` in your editor. It must declare `package guard.user`:

```rego
package guard.user

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# Read verbs that are always safe regardless of mode.
read_verbs := {"get","describe","logs","exec","port-forward","top","cluster-info",
               "config","explain","api-resources","api-versions","auth","diff","wait","version"}

# Prod cluster: block any kubectl verb that is not in read_verbs.
block contains {"reason": sprintf("kubectl %s blocked: prod is read-only", [input.verb])} if {
    input.tool == "kubectl"
    input.kubectl.cluster_name == "prod"
    not input.verb in read_verbs
}

# Dev cluster: block namespace deletion specifically.
block contains {"reason": "kubectl delete namespace blocked on dev cluster"} if {
    input.tool == "kubectl"
    input.kubectl.cluster_name == "dev"
    input.verb == "delete"
    "namespace" in input.positional
}
```

This is the policy from [`examples/policies/per-cluster.rego`](https://github.com/UndermountainCC/failsafe/blob/main/examples/policies/per-cluster.rego). Copy it as a starting point and adjust cluster names.

!!! note "User layer blocks survive write mode"
    `~/.config/failsafe/policy.rego` lives in the user layer and fires even when a pane is in `read & write` mode. Only a repo-level `allow_override` in a trusted `.failsafe.rego` can lift a user-layer block. See [Policy cascade](../explanation/policy-cascade.md).

## 3. Validate before relying on it

```bash
failsafe validate ~/.config/failsafe/policy.rego
```

Expected output:

```
✓ parse OK
✓ package: guard.user
✓ rule names: no reserved-rule violations
✓ rule shapes: all block/allow_override return {"reason": ...}
✓ fact-field references: all known

OK.
```

Fix any `✗` lines before proceeding. The `--strict` flag promotes `⚠` warnings to failures.

## 4. Dry-run a command against the policy

```bash
failsafe explain "kubectl --context prod apply -f manifests/"
```

Expected:

```
── call 1: kubectl ──
Verb:        apply
Effective cwd: /your/cwd
Mode:        read
Policy chain (2 modules at this cwd):
  [bundled] bundled/kubectl.rego
  [user] /Users/you/.config/failsafe/policy.rego
Decision: BLOCK
Reason  : kubectl apply blocked: prod is read-only
```

If you see `ALLOW` instead, the cluster name in the policy does not match the resolved `cluster_name`. Run `kubectl config view` to check the exact context name.

## 5. Confirm the policy is loaded

```bash
failsafe policies list
```

The user policy file should appear under `[user]`.

## See also

- [Explain a command](./explain-a-command.md)
- [Protect a repo with a checked-in policy](./repo-policy.md)
- [Policy cascade](../explanation/policy-cascade.md)
- [Fact schema](../reference/fact-schema.md)
