<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# iTerm2: per-session mode toggle (Python API)

failsafe resolves the current session's mode from `~/.claude/pane-mode/$ITERM_SESSION_ID`
(see the mode-source chain in the README). The goal: bind a key (say `Ctrl+Opt+T`) that
flips the **focused** session between `enabled` and `disabled` without injecting text
into your shell.

> **The one catch.** iTerm2's Python API gives a script the session's *GUID*
> (`iterm2.Reference("id")`), but failsafe keys the mode file on the shell's
> `$ITERM_SESSION_ID` env var (format `wNtMpK:GUID`). Those differ, so we bridge them:
> the shell publishes its `$ITERM_SESSION_ID` to an iTerm **user variable**, and the
> Python toggle reads it back. One-line shell hook, done once.

If you'd rather skip Python entirely, jump to [the 30-second alternative](#alternative-no-python).

---

## 1. Shell hook: publish `$ITERM_SESSION_ID`

iTerm user variables are set with an OSC 1337 escape and a **base64** value. Add to your
`~/.zshrc` (or `~/.bashrc`):

```sh
# Bridge $ITERM_SESSION_ID to an iTerm user var so the toggle script can find this session.
if [ -n "$ITERM_SESSION_ID" ]; then
  printf '\033]1337;SetUserVar=failsafe_sid=%s\a' "$(printf %s "$ITERM_SESSION_ID" | base64 | tr -d '\n')"
fi
```

Open a new tab after adding it so the variable is set.

## 2. The toggle script

Save as `~/Library/Application Support/iTerm2/Scripts/AutoLaunch/failsafe_toggle.py`
(create the `AutoLaunch` folder if needed). It registers a function iTerm can bind to a key.

```python
#!/usr/bin/env python3
# failsafe per-session mode toggle for iTerm2.
# Flips ~/.claude/pane-mode/$ITERM_SESSION_ID between "enabled" and "disabled".
import base64
import os
import iterm2

MODE_DIR = os.path.expanduser("~/.claude/pane-mode")

def read_mode(path):
    try:
        with open(path) as f:
            return (f.read().strip() or "enabled")
    except FileNotFoundError:
        return "enabled"            # missing file = safe default

async def main(connection):
    # `sid_b64` is the base64 of $ITERM_SESSION_ID, published by the shell hook in step 1.
    # The trailing '?' makes the reference optional (None if a session has no hook yet).
    @iterm2.RPC
    async def failsafe_toggle(sid_b64=iterm2.Reference("user.failsafe_sid?")):
        if not sid_b64:
            return
        sid = base64.b64decode(sid_b64).decode()
        os.makedirs(MODE_DIR, exist_ok=True)
        path = os.path.join(MODE_DIR, sid)

        current = read_mode(path)
        new_mode = "disabled" if current == "enabled" else "enabled"
        with open(path, "w") as f:
            f.write(new_mode)     # canonical value the Rego policies match

        # macOS notification (optional — comment out if you don't want it)
        os.system(
            "osascript -e 'display notification "
            f'"{current}  →  {new_mode}" with title "🔒 failsafe"\' >/dev/null 2>&1'
        )

    await failsafe_toggle.async_register(connection)

iterm2.run_forever(main)
```

Make sure the iTerm2 Python runtime is installed (iTerm2 → **Scripts → Manage →
Install Python Runtime**), then **Scripts → failsafe_toggle.py** to launch it (AutoLaunch
runs it automatically on future starts).

## 3. Bind the key

iTerm2 → **Settings → Keys → Key Bindings → +**:

- **Keyboard Shortcut:** `Ctrl+Opt+T` (or your pick)
- **Action:** `Invoke Script Function`
- **Function call:** `failsafe_toggle()`

Press it in any session. The focused session's mode flips and (optionally) a macOS
notification shows the change. Confirm with `failsafe mode get`.

---

## Alternative (no Python)

If you don't want the API script, bind a key to run the command in-shell. It works
because the shell already has `$ITERM_SESSION_ID`:

iTerm2 → **Settings → Keys → Key Bindings → +**:

- **Keyboard Shortcut:** `Ctrl+Opt+T`
- **Action:** `Send Text`
- **Text:** `failsafe toggle\n`

Tradeoff: this types the command into the current line, so only use it at an empty
prompt (it injects mid-command if something is running). The Python approach above
toggles silently regardless of what the shell is doing.


## "sudo mode"

Disabling failsafe is your `sudo`, so make the notification say so. Swap the `osascript`
line in the script for:

```python
title = "🔓 failsafe: sudo mode" if new_mode == "disabled" else "🔒 failsafe"
sub   = "write enabled — with great power…" if new_mode == "disabled" else "back to enabled"
os.system(f"osascript -e 'display notification \"{sub}\" with title \"{title}\"' >/dev/null 2>&1")
```

See the WezTerm guide's *"sudo mode"* section for the matching badge and the **sudo
timeout** trick (auto-revert to enabled after N minutes; the same idea works here:
`os.system(f\"( sleep 600; echo enabled > '{path}' ) &\")` right after the write).

## Notes

- The file always stores the **canonical** value (`enabled` / `disabled`), the same
  thing `failsafe mode set on` / `off` normalize to, so the toggle and the CLI agree.
- Per-session isolation depends on `$ITERM_SESSION_ID` being unique per session, which
  iTerm guarantees. If a session predates the step-1 hook, open a fresh tab.
