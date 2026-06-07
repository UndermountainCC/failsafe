package failsafe.bundled.helm

import future.keywords.if
import future.keywords.in
import future.keywords.contains

read_verbs := {"list", "get", "status", "show", "search", "version", "history", "template"}

# Block all non-read verbs while failsafe is enabled. `repo` requires looking at subverb.
block contains {"reason": sprintf("helm %s blocked while failsafe is enabled", [input.verb])} if {
    not input.failsafe_enabled == false
    input.tool == "helm"
    input.verb != ""
    input.verb != "repo"
    not input.verb in read_verbs
}

block contains {"reason": sprintf("helm repo %s blocked while failsafe is enabled", [input.subverb])} if {
    not input.failsafe_enabled == false
    input.tool == "helm"
    input.verb == "repo"
    input.subverb != "list"
}
