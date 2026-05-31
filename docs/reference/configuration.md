<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Configuration Reference

All file paths and environment variables that configure failsafe — each with its default value and effect.

---

## File paths

All paths under `~/.config/failsafe/` are created by failsafe as needed (mode 0700 for the directory, 0600 for log files). Paths under `~/.claude/` are shared with Claude Code.

### Policy files

| Path | Layer | Description |
|------|-------|-------------|
| `~/.config/failsafe/policy.rego` | User | User-level Rego policy. Must declare `package guard.user`. May contain `block` rules only — `allow_override` is reserved for the repo layer. Loaded on every hook call; missing file is silently skipped. |
| `.failsafe.rego` | Repo | Repository-level Rego policy, placed at the repo root. Must declare `package guard.repo`. May contain both `block` and `allow_override` rules. Ignored until the repo is trusted via `failsafe trust`. Discovered by walking up from the effective `cwd` toward `$HOME` (exclusive). Multiple `.failsafe.rego` files in nested directories are all loaded. |

### Tool definitions

| Path | Description |
|------|-------------|
| `~/.config/failsafe/tools/` | Directory of user-supplied YAML tool definitions (`*.yaml`). Each file defines a new tool parser — binary name(s), global flags, verb/subverb list. Loaded alongside the bundled tool definitions on every hook call. A malformed file is a fatal error (fail-closed). |

See `examples/tools/` in the repo for YAML tool templates.

### Audit log

| Path | Default | Description |
|------|---------|-------------|
| `~/.config/failsafe/decisions.jsonl` | Default when `FAILSAFE_LOG` is unset and `$HOME` is set. | JSON Lines audit log. One record per infra-tool decision (`block`, `allow`, `allow_override`) and per refuse/parse block. Not written for non-infra commands (`ls`, `echo`, etc.). Secrets in command strings are redacted before writing. File permissions: 0600; directory: 0700. |

#### `decisions.jsonl` record fields

| JSON key | Type | Description |
|----------|------|-------------|
| `ts` | string | UTC timestamp, RFC3339. |
| `decision` | string | `"block"`, `"allow"`, or `"allow_override"`. |
| `reason` | string | Block reason or override reason. Omitted for plain allows. |
| `mode` | string | Mode at decision time: `"read"` or `"read & write"`. |
| `tool` | string | Registry tool name. Omitted for refuse/parse blocks. |
| `verb` | string | Parsed verb. Omitted when empty or not applicable. |
| `subverb` | string | Parsed subverb. Omitted when empty. |
| `cwd` | string | Effective working directory for this call. |
| `command` | string | Raw command string, with secrets redacted to `***`. |
| `session.agent_type` | string | `"claude-code"` (the only agent type currently logged). |
| `session.agent_session_id` | string | Claude Code session ID from the hook envelope. |
| `session.terminal_pane` | string | Terminal pane identifier (the value of whichever pane env var was set: `WEZTERM_PANE`, `TMUX_PANE`, etc.). |

**Redaction patterns** — the following patterns in `command` are masked to `***` before logging:

- `--<flag-with-secret-name>=<value>` — any flag whose name contains `token`, `secret`, `password`, `passwd`, `credential`, `api-key`, `apikey`, `auth`, or `bearer`.
- `--<flag-with-secret-name> <value>` — same, space-separated form.
- `KEY=<value>` — any environment assignment whose key contains the same terms.

### Trust list

| Path | Description |
|------|-------------|
| `~/.config/failsafe/trusted-repos.yaml` | YAML list of trusted repo root paths, with `added_at` timestamps and optional `reason` annotations. Managed by `failsafe trust`. A missing file means no repos are trusted. |

### Mode files

| Path | Description |
|------|-------------|
| `~/.claude/pane-mode/<WEZTERM_PANE>` | Per-pane mode file for WezTerm. Created/updated by `failsafe toggle` / `mode set` when `WEZTERM_PANE` is set. Content: `read` or `read & write`. |
| `~/.claude/pane-mode/<TMUX_PANE>` | Per-pane mode file for tmux. |
| `~/.claude/pane-mode/<ITERM_SESSION_ID>` | Per-pane mode file for iTerm2. |
| `~/.claude/pane-mode/<KITTY_WINDOW_ID>` | Per-pane mode file for Kitty. |
| `~/.claude/pane-mode/<CLAUDE_SESSION_ID>` | Per-session mode file keyed on Claude's session ID. |
| `~/.config/failsafe/tty-<device_id>` | Per-controlling-terminal mode file. Created when no multiplexer env var is set. The `<device_id>` is the decimal `Rdev` of `/dev/tty`. |
| `~/.config/failsafe/mode` | Global fallback mode file. Read when no higher-priority source resolves. |

All mode files contain a single line (`read` or `read & write`). For the full resolution order, see [Modes](modes.md).

---

## Environment variables

| Variable | Default | Effect |
|----------|---------|--------|
| `FAILSAFE_MODE` | (unset) | When set to `read` or `read & write`, overrides all file-based mode sources. Highest priority in the chain. Not affected by `failsafe toggle` or `mode set`. |
| `FAILSAFE_LOG` | (unset) | Controls where decisions are logged. Set to `off` to disable logging entirely. Set to an absolute path to log to that file instead of the default `~/.config/failsafe/decisions.jsonl`. |
| `HOME` | (OS-provided) | Used to resolve all `~/.config/failsafe/` and `~/.claude/` paths. Set explicitly in test environments. |
| `WEZTERM_PANE` | (unset) | Pane identifier for WezTerm. Enables priority-2 mode source `~/.claude/pane-mode/${WEZTERM_PANE}`. |
| `TMUX_PANE` | (unset) | Pane identifier for tmux. Enables priority-3 mode source `~/.claude/pane-mode/${TMUX_PANE}`. |
| `ITERM_SESSION_ID` | (unset) | Session identifier for iTerm2. Enables priority-4 mode source. |
| `KITTY_WINDOW_ID` | (unset) | Window identifier for Kitty. Enables priority-5 mode source. |
| `CLAUDE_SESSION_ID` | (unset) | Claude Code session identifier. Enables priority-6 mode source. |

### `FAILSAFE_LOG` values

| Value | Behaviour |
|-------|-----------|
| unset | Log to `~/.config/failsafe/decisions.jsonl` (when `HOME` is set). |
| `off` | Logging disabled; no file is created or appended to. |
| `<absolute-path>` | Log to the specified file. The directory is created if it does not exist. |

---

## Required Claude Code hook configuration

To wire failsafe as a `PreToolUse` hook in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "failsafe hook" }]
      }
    ]
  }
}
```

The bare `"command": "failsafe"` also works because `hook` is the default subcommand.

To wire the MCP server (gives the agent `check_mode` and `toggle_mode` tools), add `failsafe mcp` to your MCP server configuration. See `docs/how-to/claude-code-hook.md` for the complete setup.
