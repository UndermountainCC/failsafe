<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Configuration Reference

failsafe is configured through a layered system: command-line flags override environment
variables, which override the config file, which overrides compiled defaults.

```
flags > env > ~/.config/failsafe/config.yaml > defaults
```

A missing config file is equivalent to all-defaults. You only need a config file if you
want to change something from the default.

---

## Config file

**Path:** `~/.config/failsafe/config.yaml`

The directory and file are created automatically at mode `0700`/`0600` on first use.
The path is fixed in code; the config file cannot repoint itself.

### Full example (all defaults)

```yaml
mode:
  pane_dir: ~/.claude/pane-mode

log:
  enabled: true
  path: ~/.config/failsafe/decisions.jsonl
  redact: true                  # safety-fixed; false is a fatal error

telemetry:
  enabled: false                # opt-in only; v1 exporter is a stub (no-op)
  otlp_endpoint: ""

policy:
  user_path: ~/.config/failsafe/policy.rego
  tools_dir: ~/.config/failsafe/tools

trust:
  path: ~/.config/failsafe/trusted-repos.yaml
```

### Key notes

- **`mode.default` does not exist.** The default guard mode is hardcoded `enabled`; it
  is not configurable. A configurable default would be a self-disable vector (an agent
  editing config.yaml could bypass the guard).
- **`log.redact` is safety-fixed `true`.** Setting it to `false` is a fatal error at
  load time.
- **`control_plane.*` is reserved in v1.** Setting either field is a fatal error at load
  time.
- Tilde (`~`) and `${VAR}` are expanded in all path fields after loading.

---

## Environment variable bindings

FAILSAFE_* env vars are loaded after the config file and override it.

### Config-backed env vars

| Variable | Config key | Default | Effect |
|----------|-----------|---------|--------|
| `FAILSAFE_MODE_PANE_DIR` | `mode.pane_dir` | `~/.claude/pane-mode` | Toggle-file directory. |
| `FAILSAFE_LOG_ENABLED` | `log.enabled` | `true` | `false` disables all logging. |
| `FAILSAFE_LOG_PATH` | `log.path` | `~/.config/failsafe/decisions.jsonl` | Audit log file path. |
| `FAILSAFE_LOG_REDACT` | `log.redact` | `true` | Safety-fixed; `false` is a fatal error. |
| `FAILSAFE_TELEMETRY_ENABLED` | `telemetry.enabled` | `false` | Opt-in telemetry. v1 exporter is a no-op stub. |
| `FAILSAFE_TELEMETRY_OTLP_ENDPOINT` | `telemetry.otlp_endpoint` | `""` | OTLP collector URL (unused in v1). |
| `FAILSAFE_POLICY_USER_PATH` | `policy.user_path` | `~/.config/failsafe/policy.rego` | User Rego policy path. |
| `FAILSAFE_POLICY_TOOLS_DIR` | `policy.tools_dir` | `~/.config/failsafe/tools` | User tool definition directory. |
| `FAILSAFE_TRUST_PATH` | `trust.path` | `~/.config/failsafe/trusted-repos.yaml` | Trusted-repos list path. |

### Back-compat env vars (unchanged behavior)

These two variables predate the config file and are handled by special shims so their
existing behavior is preserved exactly. They are **not** direct config-key mappings.

| Variable | Default | Effect |
|----------|---------|--------|
| `FAILSAFE_MODE` | (unset) | When set to `enabled` or `disabled`, overrides all file-based mode sources. Highest priority in the mode chain. Not mapped to a config struct field. Not affected by `failsafe toggle` or `mode set`. |
| `FAILSAFE_LOG` | (unset) | Legacy log control. `off` → logging disabled. Any other non-empty value → treated as an absolute path (sets `log.path` and `log.enabled=true`). Takes precedence over `FAILSAFE_LOG_PATH` / `FAILSAFE_LOG_ENABLED` when set. |

### `FAILSAFE_LOG` values

| Value | Behaviour |
|-------|-----------|
| unset | Log to `~/.config/failsafe/decisions.jsonl` (when `HOME` is set). |
| `off` | Logging disabled; no file is created or appended to. |
| `<absolute-path>` | Log to the specified file. Directory is created if it does not exist. |

### Other env vars

| Variable | Default | Effect |
|----------|---------|--------|
| `HOME` | (OS-provided) | Used to resolve all `~/.config/failsafe/` and `~/.claude/` paths. Set explicitly in test environments. |
| `WEZTERM_PANE` | (unset) | Pane identifier for WezTerm. Enables priority-2 mode source `~/.claude/pane-mode/${WEZTERM_PANE}`. |
| `TMUX_PANE` | (unset) | Pane identifier for tmux. Enables priority-3 mode source `~/.claude/pane-mode/${TMUX_PANE}`. |
| `ITERM_SESSION_ID` | (unset) | Session identifier for iTerm2. Enables priority-4 mode source. |
| `KITTY_WINDOW_ID` | (unset) | Window identifier for Kitty. Enables priority-5 mode source. |
| `CLAUDE_SESSION_ID` | (unset) | Claude Code session identifier. Enables priority-6 mode source. |

---

## File paths

All paths under `~/.config/failsafe/` are created by failsafe as needed (mode `0700` for
the directory, `0600` for log files). Paths under `~/.claude/` are shared with Claude Code.

### Policy files

| Path | Layer | Description |
|------|-------|-------------|
| `~/.config/failsafe/policy.rego` | User | User-level Rego policy. Must declare `package failsafe.user` (legacy `package guard.user` still accepted). May contain `block` rules only; `allow_override` is reserved for the repo layer. Loaded on every hook call; missing file is silently skipped. |
| `.failsafe.rego` | Repo | Repository-level Rego policy, placed at the repo root. Must declare `package failsafe.repo` (legacy `package guard.repo` still accepted). May contain both `block` and `allow_override` rules. Ignored until the repo is trusted via `failsafe trust`. Discovered by walking up from the effective `cwd` toward `$HOME` (exclusive). Multiple `.failsafe.rego` files in nested directories are all loaded. |

### Tool definitions

| Path | Description |
|------|-------------|
| `~/.config/failsafe/tools/` | Directory of user-supplied YAML tool definitions (`*.yaml`). Each file defines a new tool parser: binary name(s), global flags, verb/subverb list. Loaded alongside the bundled tool definitions on every hook call. A malformed file is a fatal error (fail-closed). |

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
| `mode` | string | Mode at decision time: `"enabled"` or `"disabled"`. |
| `tool` | string | Registry tool name. Omitted for refuse/parse blocks. |
| `verb` | string | Parsed verb. Omitted when empty or not applicable. |
| `subverb` | string | Parsed subverb. Omitted when empty. |
| `cwd` | string | Effective working directory for this call. |
| `command` | string | Raw command string, with secrets redacted to `***`. |
| `session.agent_type` | string | `"claude-code"` (the only agent type currently logged). |
| `session.agent_session_id` | string | Claude Code session ID from the hook envelope. |
| `session.terminal_pane` | string | Terminal pane identifier (the value of whichever pane env var was set: `WEZTERM_PANE`, `TMUX_PANE`, etc.). |

**Redaction patterns:** the following patterns in `command` are masked to `***` before logging:

- `--<flag-with-secret-name>=<value>`: any flag whose name contains `token`, `secret`, `password`, `passwd`, `credential`, `api-key`, `apikey`, `auth`, or `bearer`.
- `--<flag-with-secret-name> <value>`: same, space-separated form.
- `KEY=<value>`: any environment assignment whose key contains the same terms.

### Trust list

| Path | Description |
|------|-------------|
| `~/.config/failsafe/trusted-repos.yaml` | YAML list of trusted repo root paths, with `added_at` timestamps and optional `reason` annotations. Managed by `failsafe trust`. A missing file means no repos are trusted. |

### Mode files

| Path | Description |
|------|-------------|
| `~/.claude/pane-mode/<WEZTERM_PANE>` | Per-pane mode file for WezTerm. Created/updated by `failsafe toggle` / `mode set` when `WEZTERM_PANE` is set. Content: `enabled` or `disabled`. |
| `~/.claude/pane-mode/<TMUX_PANE>` | Per-pane mode file for tmux. |
| `~/.claude/pane-mode/<ITERM_SESSION_ID>` | Per-pane mode file for iTerm2. |
| `~/.claude/pane-mode/<KITTY_WINDOW_ID>` | Per-pane mode file for Kitty. |
| `~/.claude/pane-mode/<CLAUDE_SESSION_ID>` | Per-session mode file keyed on Claude's session ID. |
| `~/.config/failsafe/tty-<device_id>` | Per-controlling-terminal mode file. Created when no multiplexer env var is set. The `<device_id>` is the decimal `Rdev` of `/dev/tty`. |
| `~/.config/failsafe/mode` | Global fallback mode file. Read when no higher-priority source resolves. |

All mode files contain a single line (`enabled` or `disabled`). For the full resolution order, see [Modes](modes.md).

---

## Safety-fixed knobs

The following configuration values cannot be changed:

| Knob | Fixed value | Reason |
|------|-------------|--------|
| `log.redact` | `true` | Prevents secrets in command arguments from reaching the audit log. Disabling would silently expose credentials. |
| `control_plane.url` | (must be empty) | Reserved for v2; not supported in v1. |
| `control_plane.bundle_signing_key` | (must be empty) | Reserved for v2; not supported in v1. |
| Default guard mode | `enabled` | Hardcoded in the mode chain; not a config value. A configurable default would allow an agent to disable the guard by editing config.yaml. |

Setting a rejected value causes `failsafe` to exit immediately with a fatal error, so a
misconfigured deployment fails visibly rather than silently bypassing the guard.

---

## Config self-protection

failsafe provides several layers of protection against an agent manipulating the
configuration to weaken the guard:

1. **Config file path is fixed in code.** The path `~/.config/failsafe/config.yaml` is a
   compile-time constant; no config value can repoint the config file to another location.

2. **`Validate()` rejects fail-open knobs.** Any config file or env var that sets
   `log.redact: false` or populates `control_plane.*` is rejected with a fatal error at
   load time. The guard fails closed, not open.

3. **Default guard mode is hardcoded `enabled`.** There is no `mode.default` config key.
   The mode chain's default is the literal string `"enabled"`, set in code. An agent that
   edits config.yaml cannot write a `mode.default: disabled` line — no such key exists.

4. **Residual surface.** An attacker with shell write access to `~/.config/failsafe/`
   could disable logging (`log.enabled: false`) or repoint `pane_dir` to a different
   directory. This is part of the broader "protect sensitive files from the agent" problem
   — the same access that lets an agent edit config.yaml lets it edit `~/.aws/credentials`
   or `~/.ssh/id_rsa`. The solution is a **filesystem-access guard** (see the
   [roadmap](../explanation/roadmap.md#filesystem-access-guard)) that blocks agent writes
   to sensitive directories before they execute, rather than trying to harden individual
   files post-write.

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
