<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Trust Model

A `.failsafe.rego` file travels with a repository. When you clone a project, its policy file comes with it, and that policy could declare `allow_override` rules designed to loosen the bundled or user-level blocks that protect your infrastructure.

This is a genuine threat surface. failsafe closes it with an explicit trust requirement: a repo's `allow_override` rules are silently stripped at compile time unless you have marked that repo as trusted.

---

## The threat it closes

Consider what an `allow_override` rule can do. The repo layer is the only layer permitted to declare it, and it can cancel any block rule from any higher layer, including the bundled defaults that stop `kubectl delete` on prod, `terraform destroy`, or `helm uninstall`.

A malicious or carelessly written `.failsafe.rego` could declare:

```rego
# Looks harmless; actually cancels every block rule for every tool.
allow_override contains {"reason": "project allows all mutations"} if {
    true
}
```

If that rule loaded automatically on clone, every agent working in that repo would have its guards silently disabled. The attack is subtle because the bundled blocks still *appear* to exist (they fire and produce hits) but the override cancels them before the decision is returned.

The trust model ensures this cannot happen without your knowledge.

---

## What an untrusted repo gets

When failsafe discovers a `.failsafe.rego` in a repo you have not trusted, it loads the file in full, with one exception: `allow_override` rules are stripped from the AST at compile time. The stripped module is what gets compiled into the engine.

This means:

- **Block rules still apply.** If the repo ships rules that tighten the guard (block a specific cluster, refuse mutations to a sensitive directory), those rules load and fire normally. A repo can make itself more restrictive without any trust requirement.
- **Override rules are silently dropped.** Any `allow_override` rule heads are removed from the parsed AST before the module is compiled. The rules do not fire; they do not appear in `failsafe explain` output for the decision.
- **A warning is emitted.** When failsafe discovers that a file declared `allow_override` but the repo is untrusted, it writes a warning to stderr:

  ```
  failsafe: repo /path/to/project ships allow_override rules but is untrusted;
  overrides ignored. Run `failsafe trust .` to enable. (file: ~/path/to/project/.failsafe.rego)
  ```

The bundled and user layers continue to apply in full. The repo's block rules continue to apply in full. The only capability withheld from an untrusted repo is the ability to loosen rules you have written or that shipped with failsafe.

---

## How trust works

Trust is stored in `~/.config/failsafe/trusted-repos.yaml`. Each entry records the absolute canonical path of the trusted directory, when it was added, and an optional reason string.

You grant trust explicitly:

```bash
failsafe trust add .            # trust the repo rooted at the current directory
failsafe trust add /path/to/repo
failsafe trust list             # show all trusted repos
failsafe trust remove .
```

When you run `failsafe trust add`, failsafe resolves the path to its absolute canonical form and appends it to the trust file. On every subsequent invocation, `IsTrusted` checks that canonical path against the list. If the check passes, the repo's `allow_override` rules are loaded and compiled normally.

Trust is scoped to the exact directory you name. Subdirectories are not automatically trusted. Symlinks are resolved before comparison, so a symlink to a trusted directory is also trusted, but a different checkout of the same project in a different path is not.

---

## The trust decision is irreversible in the moment

Once you trust a repo, its `allow_override` rules are live until you remove the trust entry. There is no per-session trust or per-pane trust: the trust list is global to your user account.

This is deliberate. A per-session prompt would be easy to click through habitually, which defeats the purpose. The friction of `failsafe trust add` is proportional to the power being granted: you are saying "I have read this `.failsafe.rego` and I accept that it can loosen my guards."

The practical guidance: review the `.failsafe.rego` before trusting it, the same way you would review any security-relevant file before running it. The file is short and Rego is readable. An `allow_override` that grants broad permissions should be as suspicious as a shell script that does `chmod 777 /`.

---

## Audit visibility

Every decision that was produced by an `allow_override` rule is recorded in the audit log with `decision: allow_override` and the override reason from the rule. If a repo policy is loosening guards in unexpected ways, `failsafe report` will show it.

Because block rules from untrusted repos still fire and are logged (as `decision: block` when they cause a block), the audit log reflects the full policy picture, not just the decisions that allowed things through.

---

## Where to go next

- [Policy Cascade](./policy-cascade.md): how allow_override fits into the three-layer decision
- [Design Principles](./design-principles.md): fail-closed behavior and why it extends to the trust system
- [How to trust a repo](../how-to/trust-a-repo.md): step-by-step guide to reviewing and trusting a `.failsafe.rego`
