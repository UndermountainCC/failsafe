<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Add a custom tool parser

Teach failsafe to parse and gate a CLI tool that is not built-in by dropping a YAML file into `~/.config/failsafe/tools/`. This requires no recompile.

## How it works

At startup, failsafe loads tool parsers in this order: built-in Go parsers (`kubectl`, `helm`), bundled YAML parsers, then user YAML parsers. **Later entries win on name collision**, so a user YAML can override a bundled parser.

Each parser declares which binary names it matches, which verbs and subverbs it recognises, and which flags it knows about. failsafe uses the parser to extract a structured fact (`input.tool`, `input.verb`, `input.flags`, `input.positional`) that Rego policies evaluate.

## 1. Create the tools directory

```bash
mkdir -p ~/.config/failsafe/tools
```

## 2. Write the YAML

Create a file named `<tool>.yaml`. The filename is arbitrary: the `name` field inside the YAML is what appears in facts and reports.

Here is the complete example for the GitHub CLI from [`examples/tools/gh.yaml`](https://github.com/UndermountainCC/failsafe/blob/main/examples/tools/gh.yaml):

```yaml
name: gh
match: ["gh"]
env_prefix: true
global_flags:
  - { long: "version", takes_value: false }
  - { long: "help",    takes_value: false }
verbs:
  pr:
    subverbs: [list, view, create, close, comment, diff, edit, merge, ready, status]
  issue:
    subverbs: [list, view, create, close, comment, edit, status]
  repo:
    subverbs: [list, view, create, clone, fork, edit, archive, delete]
  workflow:
    subverbs: [list, view, run, enable, disable]
  run:
    subverbs: [list, view, watch, cancel, rerun, download]
  auth:
    subverbs: [login, logout, status, token, refresh]
```

Save it to `~/.config/failsafe/tools/gh.yaml`.

### YAML schema reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Tool name in facts and reports (`input.tool`). |
| `match` | list of strings | yes | Binary basenames that activate this parser (e.g. `["gh"]`). Absolute paths are matched on their basename. |
| `env_prefix` | bool | no | Accept `ENV=val` tokens before the binary (like `GITHUB_TOKEN=…`). |
| `global_flags` | list | no | Flags recognised before the verb. |
| `verbs` | map | no | Known verbs. Each entry may have `subverbs` (list of strings) and `flags` (list of flag defs). |
| `combine_shorts` | bool | no | Allow combined single-char flags (e.g. `-it`). |

**Flag definition fields:**

| Field | Type | Description |
|-------|------|-------------|
| `long` | string | Long flag name without `--` (e.g. `dry-run`). |
| `short` | string | Short flag character without `-` (e.g. `n`). |
| `takes_value` | bool | `true` if the flag consumes the next argument. |
| `repeated` | bool | `true` if the flag may appear multiple times. |
| `style` | string | `"gnu_short"` for single-dash long flags (terraform-style `-chdir`). |

Unknown flags in a real command are not an error: they are passed through as boolean `true` entries in `input.flags`. This is intentional. It fails closed (the verb is still extracted and policies still fire) rather than crashing on unrecognised options.

## 3. Verify the parser loaded

```bash
failsafe tools list
```

Look for your tool in the output:

```
kubectl       (built-in Go)
helm          (built-in Go)
terraform     (bundled YAML)
aws           (bundled YAML)
git           (bundled YAML)
gh            (user YAML at /Users/you/.config/failsafe/tools/gh.yaml)
```

## 4. Write a policy to gate the new tool

A loaded parser alone does not block anything: it only makes facts available. Add a `block` rule to your user policy (`~/.config/failsafe/policy.rego`) to act on the new tool:

```rego
package failsafe.user

import future.keywords.if
import future.keywords.contains

# Block gh repo delete from agent panes.
block contains {"reason": "gh repo delete is irreversible — run manually"} if {
    input.tool == "gh"
    input.verb == "repo"
    input.subverb == "delete"
}
```

## 5. Dry-run to confirm

```bash
failsafe explain "gh repo delete my-org/my-repo"
```

```
── call 1: gh ──
Verb:        repo
Subverb:     delete
Effective cwd: /Users/you
Mode:        read
Policy chain (2 modules at this cwd):
  [bundled] bundled/kubectl.rego
  [user] /Users/you/.config/failsafe/policy.rego
Decision: BLOCK
Reason  : gh repo delete is irreversible — run manually
```

## See also

- [Write a per-cluster user policy](./per-cluster-policy.md)
- [Explain a command](./explain-a-command.md)
- [CLI reference: tools list](../reference/cli.md#tools-list)
- [Fact schema](../reference/fact-schema.md)
