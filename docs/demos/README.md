<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Documentation demos (asciicast automation)

This directory documents the **recording** half of the docs flow. It pairs with
the doc-validation harness (`test/docs/`):

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
scripts/record-docs.sh --claude   # planned: also record the inside-Claude demo (see below)
```

If `virtui` is not installed the script **skips cleanly** (exit 0) with an
install hint — it never fails a build just because the recorder is absent, the
same "missing tool ⇒ skip, never a vacuous pass" rule the harness uses.

`COLS`/`ROWS` set the recorded terminal size, `STEP_PAUSE` sets the linger
between commands (`STEP_PAUSE=0` = fast). `TYPE_DELAY` sets the per-keystroke
typing speed baked into the cast itself — default `0.05` (seconds/keystroke),
`TYPE_DELAY=0` types instantly. Unlike playback speed, `TYPE_DELAY` is baked in
at record time and can't be recovered by re-rendering.

## Scenarios

| Cast | Command | Documents |
|------|---------|-----------|
| `explain-block.cast` | `failsafe explain "kubectl … delete ns payments"` | tutorials/getting-started, README — the guard blocking a destructive command |
| `mode-toggle.cast` | `failsafe mode get / set disabled / set enabled` | reference/modes, getting-started step 4 |
| `audit.cast` | `failsafe audit` | how-to/repo-policy — the policy chain in force |
| `tmux-hotkey.cast` | `failsafe explain` blocked, then `Ctrl+Alt+T` pressed live through tmux | toggle/tmux — the keyboard toggle flipping block → allow → re-arm |

Add a scenario by appending one `record_scenario` block in the script; keep the
command byte-identical to the doc snippet it illustrates.

## Planned: inside-Claude demo (`--claude`)

The `--claude` flag exists in the script, but the inside-Claude scenario
itself is still being redesigned — no `.cast`/`.svg` for it exists yet, and
nothing in `docs/` references one. When it ships:

- **Uses your real `$HOME`.** Claude Code auth lives in the Keychain and
  `~/.claude`, neither of which can be sandboxed the way `mode-toggle` and
  `explain-block` are. `--claude` opts into that instead of forcing it on
  everyone who runs the script.
- **Runs with your own real global config, unmodified.** The recorded Claude
  session is not stripped down or given special instructions to skip its own
  review steps. If another guard — a project hook, a permission prompt, a
  Claude-side safety check — fires before failsafe does, that's the honest,
  layered result, and that's what gets recorded.
- **Spends a few tokens per re-record.** It's a real Claude Code turn against
  the real API, not a mock.
- **Never runs in CI.** Only invoked on demand, by hand, when the docs need a
  refresh.

The recording ships once the scenario is reliable.

## What is **not** automated (honest split)

virtui is headless/VT100, so most GUI-only surfaces stay out of reach —
WezTerm toasts & `format-tab-title`, iTerm2's Python-runtime keybinding are
not recordable here and remain **manual**, exactly the STATIC / LIVE-MANUAL
surfaces called out in `test/docs/REPORT.md`.

The tmux hotkey is the exception: virtui drives a real PTY, so it can press
`Ctrl+Alt+T` *through tmux's root key table* — the same path a human's
keyboard takes, and something even the bats harness can't do (see the
`send-keys` comment in `test/docs/tmux.bats`: injecting keys into the pane
app bypasses the root key table, so `bind -n` bindings can only be checked by
asserting the binding is registered, not by firing it). `tmux-hotkey.cast` is
the first cast in this suite that shows a **live keybinding firing**, not
just its downstream effect.

This script automates the CLI and tmux documentation flow, not the full
WezTerm/iTerm2 flow.

## Requirements

| Tool | Purpose | Install |
|------|---------|---------|
| [virtui](https://github.com/honeybadge-labs/virtui) | record the sessions | `go install github.com/honeybadge-labs/virtui/cmd/virtui@latest` |
| [agg](https://github.com/asciinema/agg) | `.cast` → `.gif` (optional) | `brew install agg` |
| [svg-term-cli](https://github.com/marionebl/svg-term-cli) | `.cast` → `.svg` (optional) | `npm i -g svg-term-cli` |

> **Status:** prototype. virtui is early-stage (v0.2.0). The recorder is wired as
> an **on-demand** tool (and a `workflow_dispatch`-only CI job), not a required
> gate, so an unstable upstream can never block `main`.
