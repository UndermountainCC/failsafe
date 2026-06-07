# Doc validation findings — 2026-06-02

Validates the instructions in `docs/toggle/wezterm.md`, `docs/toggle/iterm.md`,
`docs/toggle/tmux.md`, and `docs/claude-statusline.md` against the shipped `failsafe`
binary by running the **literal fenced code blocks extracted from the docs**.

- **Headless suite** (`make validate-docs`, 34 tests): green in CI
  (`.github/workflows/doc-validation.yml`, ubuntu-latest) and locally.
- **Live GUI pass** (`make validate-docs-live`): real WezTerm.
- Binary: `failsafe 0.0.0-dev` (local) / built from source in CI.
- Local env: Darwin arm64 · tmux 3.6b · WezTerm 20240203 · Lua 5.5 (brew) · CI uses
  Ubuntu tmux + lua5.4.

## Per-claim results

| Doc | Claim | Method | Result |
|---|---|---|---|
| cross-cutting | chain order WEZTERM→TMUX→ITERM | black-box `mode get` w/ competing files | **PASS** |
| cross-cutting | missing file ⇒ `enabled` | black-box | **PASS** |
| cross-cutting | rego gates on boolean `failsafe_enabled` (not `input.mode` string) | grep `internal/embed/policies/*.rego` | **PASS** |
| cross-cutting | on/off aliases ⇒ `enabled`/`disabled` canonical bytes | `mode set` + read back | **PASS** |
| cross-cutting | `mode get` tab-delimited (`cut -f1`) returns `enabled`/`disabled` | black-box | **PASS** |
| statusline | `🔒 enabled` / `🔓 disabled` glyphs | pipe JSON to `examples/claude-statusline.sh` | **PASS** |
| statusline | jq adds `~`-cwd + model | with jq | **PASS** |
| statusline | degrades w/o jq (guard only, no cwd) | nojq PATH shim | **PASS** |
| statusline | single-line output | byte check | **PASS** |
| tmux | toggle script flips enabled↔disabled | run extracted script (via `run-shell`) | **PASS** |
| tmux | `#{pane_id}` == `$TMUX_PANE` | live headless tmux session | **PASS** |
| tmux | `C-M-t` bound to toggle w/ `#{pane_id}` | `list-keys -T root` (registration) | **PASS** |
| tmux | status script colors (sudo/amber, on/green) | run extracted script | **PASS** |
| tmux | no-script `failsafe toggle` toggles target pane | black-box | **PASS** |
| wezterm | snippet is valid lua | `lua` loadfile | **PASS** |
| wezterm | snippet has no luacheck errors | luacheck (runs in CI; skipped local, see Notes) | **PASS** (CI) |
| wezterm | snippet's own `toggle_mode` writes canonical; failsafe agrees | lua stub + driver fires `keys[1].action` | **PASS** |
| wezterm | badge maps disabled→`off`, enabled→`on` | exact-grep doc line | **PASS** |
| wezterm | sudo-timeout auto-revert mechanism | text-ties to doc + ported 1s revert | **PASS** |
| wezterm | toast / `format-tab-title` rendering | — | **STATIC** (GUI-only) |
| wezterm | config loads + `Ctrl+Alt+t` registered | `wezterm show-keys` (live, real WezTerm) | **PASS (LIVE)** |
| iterm | shell hook OSC-1337 base64 roundtrip | run extracted hook + `base64 -d` | **PASS** |
| iterm | doc's own `read_mode` canonical/default (`enabled`) | exec the AST `FunctionDef` | **PASS** |
| iterm | script `py_compile`s + `import iterm2` | python | **PASS** |
| iterm | script passes pyflakes | `python3 -m pyflakes` | **PASS** (after fix below) |
| iterm | no-python `failsafe toggle` flips session file | black-box | **PASS** |
| iterm | Python runtime registration + keypress | manual (`live-gui/iterm-register.md`) | **LIVE-MANUAL** |

## Doc bugs found

- **`docs/toggle/iterm.md` — unused `app` (FIXED).** pyflakes flagged
  `local variable 'app' is assigned to but never used`: the Python toggle called
  `app = await iterm2.async_get_app(connection)` but registered its RPC off `connection`
  and never used `app`. Dead API call — **removed** (commit `083ead8`). The harness's
  pyflakes check now guards against regression.
- **`docs/toggle/tmux.md` — sudo-timeout prose said `echo read` (FIXED, Phase 7).** The
  prose note at the end of the Status indicator section referenced
  `( sleep 600; echo read > "$file" ) &` as the auto-revert snippet, but the renamed
  canonical value is `enabled`. Updated to `echo enabled` (consistent with wezterm.md which
  already had the correct `echo enabled` in its lua code block).

## Benign findings (no change made)

- **`docs/toggle/wezterm.md` — `local act = wezterm.action` is unused.** luacheck reports
  it as a *warning* (not an error). It's the conventional WezTerm boilerplate alias that
  WezTerm's own docs use as an extension point, so it is intentionally retained. The
  luacheck test gates on **errors only** (0 errors), so this warning does not fail
  validation — correctness is enforced, style is not.

## Notes / portability

- **luacheck local skip.** Homebrew installs Lua 5.5; luacheck 1.2.0 crashes under it
  (`attempt to assign to const variable`). The wezterm luacheck test therefore *skips*
  locally (honestly, with a reason — never a vacuous pass) and runs for real in CI on
  lua5.4, where it passes. Verified locally against a separately-built Lua 5.4 luacheck.
- **tmux keybinding can't be fired headlessly.** `send-keys` injects into the pane app
  and bypasses tmux's root key table, so a `bind -n` can't be triggered from a script.
  The test validates *registration* (`list-keys`) + direct script execution instead —
  not a synthesized keypress.
- **tmux modifier-order portability.** `list-keys` renders the key as `C-M-t` (tmux 3.6)
  or `M-C-t` (older tmux); the test accepts either and asserts the stable
  toggle-path + `#{pane_id}` first.
- **`tmux-toggle.sh` outside tmux.** Under `set -euo pipefail` its trailing
  `tmux display-message` exits non-zero when run from a plain shell (after the mode file
  is already written). In documented use it's always invoked from a tmux keybinding, so
  this is benign — but a user running the script by hand from a non-tmux shell will see a
  spurious non-zero exit. Minor robustness note, not a bug.
- **GUI-only surfaces are STATIC/LIVE-MANUAL, never dressed as automated:** WezTerm
  toasts + `format-tab-title` rendering, and iTerm2's Python-runtime keybinding.
