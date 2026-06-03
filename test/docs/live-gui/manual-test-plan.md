# Manual GUI test plan — toggle + statusline docs

Closes the **STATIC / LIVE-MANUAL** rows in `../REPORT.md`. The automated suite proved the
file-contract logic; this proves the GUI wiring and that the docs' own setup steps work for
a human following them verbatim.

**Method:** for each environment, follow the doc's setup section exactly, then run the
checks. Record `PASS` / `FAIL` (+ notes) in the boxes. A check is PASS only if the observed
result matches "Expect" exactly. Verify mode with `failsafe mode get` (prints
`<value>\t(file: <path>)`).

Tip: keep a spare pane open running `watch -n1 failsafe mode get` (or just re-run it) to
watch the file flip in real time.

---

## Recording a demo video (macOS)

To capture the keypress → mode-flip → toast/badge live:

- **Easiest — `Cmd+Shift+5`:** the capture toolbar appears → choose **Record Entire Screen**
  (important: toasts appear top-right, so a window-only crop would miss them) → **Options**
  set "Save to" = Desktop, timer = None → **Record**. Do the keypresses, then click the
  **stop** square in the menu bar (or `Cmd+Ctrl+Esc`). The `.mov` lands on your Desktop.
- **CLI alternative:** `screencapture -v ~/Desktop/failsafe-demo.mov` then press `Esc` (or
  `Ctrl+C`) to stop. Records the full screen.
- **Show the keypresses on screen** (neither QuickTime nor `Cmd+Shift+5` does this): install
  **KeyCastr** — `brew install --cask keycastr` → launch → grant **Privacy & Security →
  Accessibility → KeyCastr**. Use its **"Command keys only"** mode (shows modifier combos
  like `⌃⌥T`, ignores normal typing) and park the overlay **bottom-center** so it doesn't
  collide with the top-right toast.
- **Framing tip:** arrange the test window so the `watch -n 1 failsafe mode get` pane and the
  tab badge are both visible, keep the top-right corner in frame for the toast, and the
  KeyCastr overlay at the bottom. Press the key a few times so the flip is obvious.
- To shrink/convert for sharing: `ffmpeg -i failsafe-demo.mov -vf scale=1280:-2 demo.mp4`
  (or export a GIF). `ffmpeg` via `brew install ffmpeg` if needed.

---

## A. WezTerm — `docs/toggle/wezterm.md`

Setup (a test config was generated for you at `~/.config/wezterm-failsafe-test/`, built from
the doc's own Drop-in snippet via `extract.sh` — your real WezTerm config is untouched):

1. **Launch an isolated test window:**
   ```
   wezterm-gui --config-file ~/.config/wezterm-failsafe-test/wezterm.lua
   ```
2. **Split it** (default `Ctrl+Shift+Alt+"` for a vertical split, or open a 2nd tab with
   `Ctrl+T`) and in one pane start a **live watcher**:
   ```
   watch -n 1 failsafe mode get
   ```
3. **In the other pane, press `Ctrl+Alt+T`** and watch the `watch` pane flip
   `read` ⇄ `read & write` live, and the tab badge flip ` r ` ⇄ ` rw `.

> **Toast not appearing?** That's macOS notification permissions, not failsafe — see
> *Troubleshooting* at the bottom. The mode-flip in the `watch` pane is the real proof;
> the toast is cosmetic (the doc marks it "optional").

- [ ] **A1 — keypress fires the toggle + toast.** Press `Ctrl+Alt+T` in a pane.
  Expect: a toast titled `🔒 failsafe` with body `read  →  read & write`; `failsafe mode get`
  now prints `read & write` with a `…/pane-mode/<WEZTERM_PANE>` path. Press again → toast
  `read & write  →  read`, mode back to `read`.
- [ ] **A2 — tab-title badge flips.** With the badge block active, look at the tab title.
  Expect: ` r ` (dim grey) when read, ` rw ` (amber) when writable; flips when you toggle.
- [ ] **A3 — per-pane isolation.** Split the window (two panes). Toggle pane 1 to write.
  Expect: `failsafe mode get` in pane 1 = `read & write`, in pane 2 = `read` (each keyed by
  its own `WEZTERM_PANE`).
- [ ] **A4 — "sudo mode" variant.** Swap in the *"sudo mode"* `toggle_action` and the
  `⚡ sudo` badge. Toggle to write. Expect: toast `🔓 failsafe: sudo mode` /
  `write enabled — with great power…`; badge shows `⚡ sudo`. Toggle off → toast
  `🔒 failsafe` / `back to read-only. phew.`
- [ ] **A5 — sudo timeout auto-revert.** Add the auto-revert snippet but use **`sleep 20`**
  (not 600) for the test. Toggle to write, then leave it. Expect: after ~20s `failsafe mode
  get` returns to `read` on its own (badge flips back on the next tab render; no toast fires
  on the auto-revert — it's a background file write). Restore `600` afterwards.

## B. iTerm2 (Python path) — `docs/toggle/iterm.md` §1–3

Setup: add the **shell hook** to `~/.zshrc`; open a new tab. Install the Python runtime
(**Scripts → Manage → Install Python Runtime**), save `failsafe_toggle.py` to
`…/iTerm2/Scripts/AutoLaunch/`, launch it (**Scripts → failsafe_toggle.py**), check
**Scripts → Console** for errors. Bind `Ctrl+Opt+T` → **Invoke Script Function** →
`failsafe_toggle()`.

- [ ] **B0 — script registers cleanly.** After launching the script, Console shows no error
  and `failsafe_toggle` is registered (it appears under Scripts).
- [ ] **B1 — keypress toggles silently + notifies.** At an empty prompt press `Ctrl+Opt+T`.
  Expect: macOS notification titled `🔒 failsafe`, body `read  →  read & write`;
  `failsafe mode get` flips. Press again → reverse.
- [ ] **B2 — no text injection.** Start typing a command (don't run it), then press
  `Ctrl+Opt+T` mid-line. Expect: mode flips, and **no text is injected** into your command
  line (this is the Python path's whole advantage over Send Text).
- [ ] **B3 — per-session isolation.** Two tabs/sessions; toggle one. Expect: `failsafe mode
  get` differs per session (keyed by each `$ITERM_SESSION_ID`).
- [ ] **B4 — "sudo mode" notification.** Swap the `osascript` line for the sudo-mode variant.
  Expect: notification title `🔓 failsafe: sudo mode` when enabling write.

## C. iTerm2 (no-Python alternative) — `docs/toggle/iterm.md` "Alternative"

Setup: bind `Ctrl+Opt+T` → **Send Text** → `failsafe toggle\n`.

- [ ] **C1 — Send Text path.** At an **empty** prompt press `Ctrl+Opt+T`. Expect: the line
  runs `failsafe toggle`, prints the `read → read & write (…path…)` transition, mode flips.
- [ ] **C2 — the documented caveat holds.** Press it while a command is half-typed. Expect:
  it injects `failsafe toggle` mid-command (confirming the doc's warning to use it only at an
  empty prompt). No need to "fix" — just confirm the caveat is real.

## D. tmux — `docs/toggle/tmux.md`

Setup: save `~/.config/failsafe/tmux-toggle.sh` and `tmux-status.sh` (`chmod +x`), add the
`bind -n C-M-t` line and the `status-right` block to `~/.tmux.conf`,
`tmux source-file ~/.tmux.conf`.

- [ ] **D1 — real keypress (what the headless test couldn't do).** In an interactive tmux
  pane press `Ctrl+Alt+T`. Expect: status message `🔒 failsafe: read → read & write`;
  `failsafe mode get` flips. Press again → reverse.
- [ ] **D2 — status bar indicator.** Watch the status-right. Expect: `🔒 read` (green) when
  read, `🔓 sudo` (amber) when writable; flips within ~2s (`status-interval 2`) of a toggle.
- [ ] **D3 — per-pane isolation.** Split a window; toggle one pane. Expect: each pane's
  `failsafe mode get` is independent (keyed by `$TMUX_PANE`).
- [ ] **D4 — nested in WezTerm/iTerm.** Run tmux inside WezTerm (or iTerm). Expect: the
  direct-write helper still targets the tmux pane correctly; the **no-script alternative**
  (`WEZTERM_PANE= ITERM_SESSION_ID= TMUX_PANE='#{pane_id}' failsafe toggle`) toggles the
  right pane and not the outer terminal.

## E. Claude Code status line — `docs/claude-statusline.md`

Setup: copy `examples/claude-statusline.sh` to `~/.config/failsafe/`, `chmod +x`, point
`~/.claude/settings.json` `statusLine.command` at it, (re)start Claude Code.

- [ ] **E1 — status line renders.** Expect: the bottom line reads
  `failsafe 🔒 read · ~/<cwd> · <Model>`.
- [ ] **E2 — end-to-end flip (the money shot).** From the same pane, toggle to write (any
  binding above, or `failsafe toggle`). Expect: the Claude Code status line flips to
  `failsafe 🔓 write · …` on its next render. This is the full chain: terminal toggle →
  guard mode file → Claude Code status line.
- [ ] **E3 — degrade without jq.** Temporarily rename/hide `jq`. Expect: the line still shows
  `failsafe 🔒 read` / `🔓 write` (just without the `· cwd · model` suffix).

---

## Results

Fill in as you go; I'll fold the outcomes (and any doc bugs found) into `../REPORT.md`.

| Check | Result | Notes |
|---|---|---|
| A1 keypress + toast | | |
| A2 tab badge | | |
| A3 per-pane isolation | | |
| A4 sudo-mode toast/badge | | |
| A5 sudo timeout revert | | |
| B0 script registers | | |
| B1 keypress + notify | | |
| B2 no text injection | | |
| B3 per-session isolation | | |
| B4 sudo-mode notification | | |
| C1 Send Text toggle | | |
| C2 injection caveat | | |
| D1 real C-M-t keypress | | |
| D2 status bar | | |
| D3 per-pane isolation | | |
| D4 nested tmux | | |
| E1 statusline renders | | |
| E2 end-to-end flip | | |
| E3 degrade w/o jq | | |

---

## Troubleshooting

- **WezTerm/iTerm toast or notification doesn't appear.** It's macOS notification
  permission, not failsafe. **System Settings → Notifications →** the app (WezTerm / iTerm /
  Script Editor for iTerm's osascript) → **Allow notifications** ON, style Banners/Alerts;
  turn off **Focus / Do Not Disturb**. The app only shows in that list after it has *tried*
  to post once, so toggle a couple times first, then re-check. The **mode flip is the real
  proof** (watch `failsafe mode get`); the toast is cosmetic — a missing toast with a working
  flip is still a PASS (note "toast = OS perms").
- **`watch` not found.** `brew install watch`, or loop manually:
  `while true; do clear; failsafe mode get; sleep 1; done`.
- **Mode doesn't flip at all.** Confirm `failsafe` is on PATH in that window
  (`failsafe --version`), and that the file is being written:
  `ls -l ~/.claude/pane-mode/`. In WezTerm the file is named by `$WEZTERM_PANE`.
- **Cleanup after testing:** the test config lives at `~/.config/wezterm-failsafe-test/`
  (delete anytime); mode files are under `~/.claude/pane-mode/` (safe to `rm`).
