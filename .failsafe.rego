# .failsafe.rego — repo policy for failsafe's own repo.
# Demonstrates: a `block` rule and an `allow_override` rule.
package guard.repo

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# This is the failsafe repo. We're our own first user — keep it readable.
# Tighten: never push --force on `main`.
# Note: --force is parsed as a flag (input.flags.force == true), not a positional.
block contains {"reason": "failsafe: never push --force on main"} if {
    input.tool == "git"
    input.verb == "push"
    input.flags.force == true
    "main" in input.positional
}

# Relax: feature branches may be force-pushed (this is normal git workflow).
allow_override contains {"reason": "failsafe: force-push on feature branches OK"} if {
    input.tool == "git"
    input.verb == "push"
    input.flags.force == true
    not "main" in input.positional
}
