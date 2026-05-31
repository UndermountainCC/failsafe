package guard.bundled.git

import future.keywords.if
import future.keywords.in

# Bundled git policy is intentionally permissive: git is allowed by default
# and individual repos use .failsafe.rego to tighten (no force-push, etc.).
# This file exists so `failsafe policies list` shows git in the bundled set.

# (No block rules — the bundled policy is "git is allowed by default".)
