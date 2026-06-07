<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# JS/TS Supply-Chain Surface

!!! warning "Roadmap: not yet shipped"
    Everything on this page describes planned future capability. **None of it is implemented today.**
    The shipped surfaces are `kubectl`, `helm`, `terraform`/`tofu`, `aws`, and `git`.
    Do not rely on any behavior described here; it does not exist yet.

---

## Two ways an agent wrecks you

When people think about an AI coding agent causing harm through shell commands, they usually picture the obvious case: the agent mutates infrastructure, deletes a namespace, destroys a Terraform stack, drops a database. The shipped surfaces in failsafe cover this attack surface well.

There is a second attack surface that is just as dangerous and considerably more subtle: **supply-chain execution**. An agent that runs `npm install` in a project directory is not (visibly) touching any infrastructure. It looks like ordinary development work. But it triggers code execution (`postinstall` scripts run automatically after packages land on disk) and the packages themselves may have been tampered with, substituted (dependency confusion), or simply malicious from the start.

An agent has no way to know the difference between a postinstall script that compiles a native module and one that exfiltrates your `~/.aws/credentials` to a remote server. It runs them both.

---

## The supply-chain surfaces

The JS/TS ecosystem has several distinct entry points for this class of attack:

### Package installs (`npm install`, `pnpm install`, `yarn`)

Running any of these commands fetches packages from a registry and executes their lifecycle scripts (`preinstall`, `install`, `postinstall`). The scripts run in your shell, with your environment variables, with access to your filesystem and network. They are not sandboxed.

The critical subtlety: a package's postinstall script is not the same as the package's published code. Registries have had incidents where a malicious postinstall script was added to a previously-legitimate package, and automated tooling (including AI agents) installed the updated version before the incident was detected.

### Remote execution via `npx`

`npx some-package` fetches and immediately executes a package that may not be installed locally. There is no install phase that a human might notice: the execution is the install. An agent told to run a scaffolding command via `npx` is silently running arbitrary remote code.

`pnpx` and `yarn dlx` are equivalent patterns.

### Workspace and monorepo amplification

In a monorepo, `npm install` at the root installs packages for every workspace. A single command can trigger postinstall scripts across dozens of packages, any one of which may be malicious or compromised.

---

## The vision

The planned approach mirrors the infrastructure surfaces: parse the command into a structured fact, evaluate it against policy, return `allow`/`deny`/`ask` before execution.

For the supply-chain surface, the structured fact would expose:

- Which package manager is being invoked and with what verb (`install`, `ci`, `add`, `dlx`, etc.)
- Whether lifecycle scripts are enabled or suppressed (`--ignore-scripts`)
- Whether a lockfile is present and would be honored
- For `npx`/`pnpx`/`yarn dlx`: the package name and version being fetched

With that fact available in Rego, a policy could express things like:

```rego
# Require --ignore-scripts whenever an agent installs packages.
block contains {"reason": "npm install without --ignore-scripts runs postinstall hooks; use --ignore-scripts in agent context"} if {
    input.tool == "npm"
    input.verb == "install"
    not input.flags["ignore-scripts"]
    not input.failsafe_enabled == false
}

# Block npx unconditionally while failsafe is enabled — it fetches and runs remote code.
block contains {"reason": "npx fetches and executes remote code; disable failsafe if you intend this"} if {
    input.tool == "npx"
    not input.failsafe_enabled == false
}
```

The repo layer could then `allow_override` for specific packages or workflows that have been reviewed.

---

## Why this matters for AI agents specifically

A human developer who runs `npm install` has typically looked at the `package.json`, noticed that a new dependency appeared, and made a judgment about whether to trust the source. An AI agent does not do this. It runs `npm install` because the task requires a dependency, without inspecting what that dependency will do when it installs.

This asymmetry, where agents are capable and fast but do not exercise skepticism about supply-chain operations, is exactly the gap that failsafe's supply-chain surface is intended to close. The guard asks the question the agent will not: *should this package be allowed to run code on your machine right now?*

---

## Current state and timeline

There is no timeline commitment for this work. The shipped surfaces (infrastructure tools) cover the most immediately dangerous commands for developers working with cloud infrastructure. The supply-chain surface is the next planned area of coverage.

If this matters for your use case, watch the [failsafe releases](https://github.com/UndermountainCC/failsafe/releases) or contribute to the design discussion in the project's issue tracker.

---

## Where to go next

- [Why failsafe](./why-failsafe.md): comprehension over matching
- [Design Principles](./design-principles.md): fail-closed behavior, which will apply to this surface too
- [Supported surfaces](../reference/bundled-policies.md): reference for what is shipped today
