<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Documentation demos (asciicast automation)

This directory documents the **recording** half of the docs flow. It pairs with
the doc-validation harness (`test/docs/`, PR #2):

| Concern | Owner | What it guarantees |
|---------|-------|--------------------|
| The copy-paste snippets in `docs/` actually run against the real binary | `test/docs/` (bats) | correctness — snippets don't drift |
| A demo *shows* those same commands running | `scripts/record-docs.sh` (virtui) | currency — the GIF/SVG matches today's binary |

Both drive the **real `failsafe` binary** from the **same commands**, so prose,
tests, and recordings stay in lock-step.

## How it works

`scripts/record-docs.sh` uses [virtui](https://github.com/honeybadge-labs/virtui)
("Playwright for the terminal") to drive `failsafe` through a real PTY and record
each session to [asciicast v2](https://docs.asciinema.org/manual/asciicast/v2/):

1. Build `failsafe` into a throwaway bindir and put it on `PATH`.
2. Point `$HOME` at a sandbox so demos never touch your real `~/.config/failsafe`
   (same isolation trick as the harness's `helpers.bash`).
3. Start the virtui daemon, then for each scenario spawn a recorded `bash`
   session, type the documented command, and `--wait` for the expected screen
   text before moving on (deterministic, no fixed `sleep`s).
4. Write `docs/assets/casts/<scenario>.cast`.
5. Optionally render each `.cast` to an embeddable `.svg`/`.gif` (`--render`) —
   GitHub markdown and MkDocs do **not** play asciicast inline.

```bash
scripts/record-docs.sh            # record .cast files
scripts/record-docs.sh --render   # also render .svg (svg-term) or .gif (agg)
```

If `virtui` is not installed the script **skips cleanly** (exit 0) with an
install hint — it never fails a build just because the recorder is absent, the
same "missing tool ⇒ skip, never a vacuous pass" rule the harness uses.

## Scenarios

| Cast | Command | Documents |
|------|---------|-----------|
| `explain-block.cast` | `failsafe explain "kubectl … delete ns payments"` | tutorials/getting-started, README — the guard blocking a destructive command |
| `mode-toggle.cast` | `failsafe mode get / set rw / set ro` | reference/modes, getting-started step 4 |
| `audit.cast` | `failsafe audit` | how-to/repo-policy — the policy chain in force |

Add a scenario by appending one `record_scenario` block in the script; keep the
command byte-identical to the doc snippet it illustrates.

## What is **not** automated (honest split)

virtui is headless/VT100, so it can only record the **CLI** surface. The
GUI-only surfaces — WezTerm toasts & `format-tab-title`, iTerm2's Python-runtime
keybinding — are not recordable here and remain **manual**, exactly the
STATIC / LIVE-MANUAL surfaces called out in `test/docs/REPORT.md`. This script
automates the CLI documentation flow, not the *full* flow.

## Requirements

| Tool | Purpose | Install |
|------|---------|---------|
| [virtui](https://github.com/honeybadge-labs/virtui) | record the sessions | `go install github.com/honeybadge-labs/virtui/cmd/virtui@latest` |
| [agg](https://github.com/asciinema/agg) | `.cast` → `.gif` (optional) | `brew install agg` |
| [svg-term-cli](https://github.com/marionebl/svg-term-cli) | `.cast` → `.svg` (optional) | `npm i -g svg-term-cli` |

> **Status:** prototype. virtui is early-stage (v0.2.0). The recorder is wired as
> an **on-demand** tool (and a `workflow_dispatch`-only CI job), not a required
> gate, so an unstable upstream can never block `main`.
