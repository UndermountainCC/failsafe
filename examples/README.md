# Examples

Drop-in starting points for failsafe policies and tool definitions.

## Policies

- `policies/per-cluster.rego` — Per-cluster kubectl rules (prod is read-only;
  dev allows mutations except namespace deletion). Place in
  `~/.config/failsafe/policy.rego` (and change the package to `failsafe.user`).
- `policies/per-repo-protection.rego` — Repo-level: kubectl apply via CI only,
  and no force-push to acme default branches. Place at
  `<repo>/.failsafe.rego`.

## Tools

- `tools/gh.yaml` — GitHub CLI. Drop in `~/.config/failsafe/tools/gh.yaml`
  to enable policy gates on `gh` commands.

## See also

`failsafe policies list` shows what's currently loaded.
`failsafe validate <path>` lints a `.rego` file before commit.
`failsafe explain <command>` dry-runs a command through the policy chain.
