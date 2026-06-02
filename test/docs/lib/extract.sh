#!/usr/bin/env bash
# extract.sh <file.md> <lang> <n> [heading_substr]
# Print the Nth (1-based) ```<lang> fenced block. If heading_substr is given,
# only blocks appearing after the most recent heading line containing that
# substring are counted (so edits elsewhere in the doc don't shift the ordinal).
# Fence detection trims surrounding whitespace (handles fences indented inside
# markdown lists); grabbed content is printed verbatim.
set -euo pipefail
file="${1:?usage: extract.sh <file.md> <lang> <n> [heading_substr]}"
lang="${2:?lang}"
want="${3:?n}"
anchor="${4:-}"

awk -v lang="$lang" -v want="$want" -v anchor="$anchor" '
  BEGIN { armed = (anchor == "") ? 1 : 0; count = 0; grab = 0; open = "```" lang }
  {
    t = $0
    sub(/^[[:space:]]+/, "", t)
    sub(/[[:space:]]+$/, "", t)
  }
  # A matching heading (re)arms and resets the counter: ordinals are relative to
  # the MOST RECENT matching heading.
  anchor != "" && /^#/ && index($0, anchor) > 0 { armed = 1; count = 0; grab = 0 }
  armed && t == open { if (grab == 0) { count++; if (count == want) { grab = 1; next } } }
  grab && t == "```" { exit }
  grab { print }
' "$file"
