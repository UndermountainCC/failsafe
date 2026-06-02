#!/usr/bin/env bash
# extract.sh <file.md> <lang> <n> [heading_substr]
# Print the Nth (1-based) ```<lang> fenced block. If heading_substr is given,
# only blocks appearing after the most recent heading line containing that
# substring are counted (so edits elsewhere in the doc don't shift the ordinal).
set -euo pipefail
file="${1:?usage: extract.sh <file.md> <lang> <n> [heading_substr]}"
lang="${2:?lang}"
want="${3:?n}"
anchor="${4:-}"

awk -v lang="$lang" -v want="$want" -v anchor="$anchor" '
  BEGIN { armed = (anchor == "") ? 1 : 0; count = 0; grab = 0 }
  anchor != "" && /^#/ && index($0, anchor) > 0 { armed = 1 }
  armed && $0 ~ ("^[[:space:]]*```" lang "[[:space:]]*$") {
    if (grab == 0) { count++; if (count == want) { grab = 1; next } }
  }
  grab && $0 ~ "^[[:space:]]*```[[:space:]]*$" { exit }
  grab { print }
' "$file"
