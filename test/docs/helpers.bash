# shellcheck shell=bash
# Shared helpers for the doc-validation bats suite.

# Repo root (this file lives at test/docs/helpers.bash).
DOCS_REPO_ROOT="${DOCS_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

# Snapshot the real HOME before any test sandboxes it (needed for python user-site imports).
ORIG_HOME="${ORIG_HOME:-$HOME}"
