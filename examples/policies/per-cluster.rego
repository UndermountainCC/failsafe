# Example: per-cluster kubectl policy. Place in ~/.config/failsafe/policy.rego
# (after dropping `package failsafe.repo` for `package failsafe.user`).
package failsafe.user

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# Prod cluster: only read verbs (mirrors bundled, but pinned to the cluster name).
read_verbs := {"get","describe","logs","exec","port-forward","top","cluster-info",
               "config","explain","api-resources","api-versions","auth","diff","wait","version"}

block contains {"reason": sprintf("kubectl %s blocked: prod is read-only", [input.verb])} if {
    input.tool == "kubectl"
    input.kubectl.cluster_name == "prod"
    not input.verb in read_verbs
}

# Dev cluster: anything except deleting namespaces.
block contains {"reason": "kubectl delete namespace blocked on dev cluster"} if {
    input.tool == "kubectl"
    input.kubectl.cluster_name == "dev"
    input.verb == "delete"
    "namespace" in input.positional
}
