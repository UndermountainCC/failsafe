# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added

- **`~/.config/failsafe/config.yaml`** — new optional config file (koanf loader).
  Precedence: `flags > env > file > defaults`. A missing file is equivalent to
  all-defaults; no migration required.

- **Config env var bindings** — `FAILSAFE_*` env vars now have documented config-key
  mappings (e.g. `FAILSAFE_LOG_PATH` → `log.path`). The existing `FAILSAFE_MODE` and
  `FAILSAFE_LOG` variables are handled by back-compat shims; their behavior is **unchanged**.

- **`internal/telemetry` package** — minimal telemetry interface, off by default.
  `New(cfg TelemetryConfig)` returns a no-op exporter when `telemetry.enabled` is
  `false` (the default) and a stub (still no-op) when enabled. No OpenTelemetry
  dependency is added in v1; the stub documents the intended OTLP payload shape for
  review before a real exporter ships.

- **Config self-protection documentation** (`docs/reference/configuration.md`) — explains
  the four protection layers: fixed config path, `Validate()` rejecting fail-open knobs,
  hardcoded default guard mode, and the residual filesystem-write surface.

- **Filesystem-access guard roadmap entry** (`docs/explanation/roadmap.md`) — describes
  the planned guard for agent writes to `~/.config/failsafe/`, `~/.aws/`, `~/.ssh/`, and
  policy files.

### Changed

- **Canonical mode vocabulary**: the two mode values are now `enabled` (failsafe active,
  bundled blocks fire) and `disabled` (failsafe bypassed, user/repo blocks still apply).
  The old strings `read` and `read & write` are retained as **legacy aliases** in the
  `mode set` input parser and the `input.mode` fact field for back-compatibility with
  existing scripts and Rego rules.

- **Primary Rego fact is now `input.failsafe_enabled` (bool)**: `true` when the mode is
  `enabled`, `false` when `disabled`. Bundled policies gate on
  `not input.failsafe_enabled == false` (fail-safe form: unknown == not-false == block).
  The legacy field `input.mode` (string, `"read"` / `"read & write"`) is retained and will
  continue to work in existing rules — no migration required.

- **Rego namespace `failsafe.*`**: bundled policies now use `failsafe.bundled.<tool>`;
  user policies use `failsafe.user`; repo policies use `failsafe.repo`. The legacy
  `guard.*` namespace (`guard.bundled.*`, `guard.user`, `guard.repo`) is still honored
  (dual-namespace) — existing `.failsafe.rego` files with `package guard.repo` do not need
  to change.

- **Expanded `mode set` aliases**: `failsafe mode set` now accepts a richer set of
  case-insensitive aliases.

  | Input | Canonical value written |
  |-------|------------------------|
  | `enabled`, `enable`, `on`, `closed`, `close`, `lock`, `ro`, `r`, `read`, `safe` | `enabled` |
  | `disabled`, `disable`, `off`, `open`, `unlock`, `rw`, `w`, `write`, `sudo` | `disabled` |

- **Default mode is `enabled`**: unchanged in behavior, but now reflected by the canonical
  string `enabled` rather than `read`.

- **Block reason prose updated**: bundled policies now emit
  `"<tool> <verb> blocked while failsafe is enabled"` instead of
  `"<tool> <verb> blocked in read mode"`.

- **Status line / badge vocabulary**: `failsafe mode get` outputs `enabled` or `disabled`;
  the bundled `claude-statusline.sh` helper renders `🔒 enabled` / `🔓 disabled`; tab
  badges in terminal snippets use `on` / `off`.
