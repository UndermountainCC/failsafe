# iTerm2 runtime registration — manual live check

iTerm2's Python toggle binds a key to `Invoke Script Function` → `failsafe_toggle()`.
That GUI registration + keypress path cannot be scripted reliably, so it is verified by
hand. The automated proxies in `../iterm.bats` already cover the rest: the OSC base64
roundtrip, the doc's own `read_mode`, `py_compile`, `import iterm2`, and pyflakes.

## Steps

1. Add the step-1 shell hook to `~/.zshrc` (or `~/.bashrc`); open a new tab.
2. iTerm2 → **Scripts → Manage → Install Python Runtime**.
3. Save the doc's `failsafe_toggle.py` to
   `~/Library/Application Support/iTerm2/Scripts/AutoLaunch/`; launch it via
   **Scripts → failsafe_toggle.py**. Confirm **Scripts → Console** shows no error.
4. iTerm2 → **Settings → Keys → Key Bindings → +**: shortcut `Ctrl+Opt+T`,
   action **Invoke Script Function**, function `failsafe_toggle()`.
5. Press the key at a shell prompt, then run `failsafe mode get` and confirm the mode
   flipped (and the notification, if enabled, appeared).

Record the result (PASS/FAIL) and the iTerm2 version in `../REPORT.md`.
