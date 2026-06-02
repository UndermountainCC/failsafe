<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Trust a repo

Activate the `.failsafe.rego` policy that lives in a repository by adding the repo's path to the trust list.

## Background

A `.failsafe.rego` file cloned from an untrusted source could loosen your guards. failsafe therefore ignores repo-level policies until you explicitly trust the path. See [Trust model](../explanation/trust-model.md) for the reasoning.

## Add a repo

From inside the repo (the command walks up from your cwd to find the nearest `.failsafe.rego`):

```bash
failsafe trust add .
```

Or pass an absolute path:

```bash
failsafe trust add /path/to/your/repo
```

On success:

```
trusted /path/to/your/repo
```

If the repo is already trusted, the command exits cleanly:

```
/path/to/your/repo is already trusted
```

!!! note "What counts as the repo root?"
    `failsafe trust add .` walks up from the current directory until it finds a `.failsafe.rego` file, stopping at `$HOME`. That directory becomes the trusted path. If no `.failsafe.rego` is found, the command prints an error. Create the policy file first (see [Protect a repo with a checked-in policy](./repo-policy.md)).

## List trusted repos

```bash
failsafe trust list
```

Each line shows the absolute path, an optional reason (if one was recorded), and the date it was added:

```
/path/to/your/repo  (added 2026-05-31)
/path/to/another-repo — added after security review  (added 2026-04-10)
```

## Remove a repo

```bash
failsafe trust remove /path/to/your/repo
```

After removal, the `.failsafe.rego` in that repo is skipped again. Bundled and user policies still apply.

## Check whether a repo is trusted

```bash
failsafe trust check
```

Checks the repo containing the current directory. Exit code `0` = trusted, `1` = not trusted:

```
/path/to/your/repo is trusted
```

Pass a path to check a specific directory:

```bash
failsafe trust check /path/to/your/repo
```

## Confirming the policy loaded

After trusting, run `failsafe explain` to confirm the repo layer appears and is marked `[trusted]`:

```bash
failsafe explain "kubectl apply -f manifests/"
```

```
Policy chain (2 modules at this cwd):
  [bundled] bundled/kubectl.rego
  [repo] /path/to/your/repo/.failsafe.rego [trusted]
```

If you still see `[untrusted]`, the path recorded in the trust list does not match the resolved repo root. Run `failsafe trust list` and compare.

## See also

- [Protect a repo with a checked-in policy](./repo-policy.md)
- [Explain a command](./explain-a-command.md)
- [Trust model](../explanation/trust-model.md)
- [CLI reference: trust](../reference/cli.md#trust)
