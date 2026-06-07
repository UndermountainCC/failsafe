# Example: protect specific repos. Place at <repo>/.failsafe.rego.
package failsafe.repo

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# This repo's manifests must be applied via CI, not local kubectl.
block contains {"reason": "this repo: kubectl apply via CI only — submit a PR"} if {
    input.tool == "kubectl"
    input.verb == "apply"
    not input.flags["dry-run"]
}

# No force-push to acme-namespaced default branches.
block contains {"reason": "no force-push to acme default branches"} if {
    input.tool == "git"
    input.verb == "push"
    "--force" in input.positional
    contains(input.git.remote_url, "acme/")
}
