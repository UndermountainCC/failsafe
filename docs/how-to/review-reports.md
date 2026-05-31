<!--
Copyright 2026 Undermountain Coding Company
SPDX-License-Identifier: CC-BY-4.0
-->

# Review audit reports

Summarise the decision log to see what failsafe has been blocking, allowing, and overriding across your guarded sessions.

## Generate a report

```bash
failsafe report
```

Prints a Markdown summary of the last 7 days by default:

```markdown
# failsafe decision report

Window: last 7d — 42 decision(s).

## Counts by tool / verb / decision

| Tool | Verb | Decision | Count |
|------|------|----------|-------|
| kubectl | get | allow | 28 |
| kubectl | apply | block | 8 |
| terraform | apply | block | 4 |
| kubectl | apply | allow_override | 2 |

## Scariest decisions

- **block** kubectl apply (score 13) in `/infra/payments` — prod is read-only
- **allow_override** kubectl apply (score 5) in `/infra/staging` — dry-run is safe
```

The **Scariest decisions** section surfaces the top-5 most alarming entries: blocks outrank overrides, and anything touching `prod` scores higher.

## Change the time window

`--since` accepts a day count (`Nd`), hours (`Nh`), or minutes (`Nm`):

```bash
failsafe report --since 24h     # last 24 hours
failsafe report --since 30m     # last 30 minutes
failsafe report --since 30d     # last 30 days
```

## Machine-readable output

```bash
failsafe report --format json
```

Returns a JSON object with `since`, `total`, a `counts` array (tool / verb / decision / count), and a `scariest` array:

```json
{
  "since": "7d",
  "total": 42,
  "counts": [
    { "tool": "kubectl", "verb": "get", "decision": "allow", "count": 28 },
    { "tool": "kubectl", "verb": "apply", "decision": "block", "count": 8 }
  ],
  "scariest": [
    {
      "ts": "2026-05-30T14:23:01Z",
      "decision": "block",
      "tool": "kubectl",
      "verb": "apply",
      "cwd": "/infra/payments",
      "reason": "prod is read-only",
      "score": 13
    }
  ]
}
```

Combine flags freely:

```bash
failsafe report --since 24h --format json | jq '.counts[] | select(.decision == "block")'
```

## Share a scrubbed report

`--share` redacts deployment-identifying data — file paths and cloud ARNs — before output, so the report is safe to paste in a ticket or share with a teammate:

```bash
failsafe report --share
failsafe report --since 24h --format json --share
```

Secrets (tokens, passwords, credential flags) are always redacted in the raw log; `--share` adds a second scrub pass for paths and ARNs.

## Where the log lives

All decisions are written to `~/.config/failsafe/decisions.jsonl`. Each line is a JSON record. Only commands involving a guarded tool (`kubectl`, `helm`, `terraform`, `aws`, `git`) are logged — `ls`, `echo`, and similar pass-through silently.

To inspect raw records:

```bash
tail -f ~/.config/failsafe/decisions.jsonl | jq .
```

## See also

- [Explain a command](./explain-a-command.md)
- [Bundled policies](../reference/bundled-policies.md)
- [CLI reference — report](../reference/cli.md#report)
