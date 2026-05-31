package guard.bundled.helm

import future.keywords.if
import future.keywords.in
import future.keywords.contains

read_verbs := {"list", "get", "status", "show", "search", "version", "history", "template"}

# Block all non-read verbs in read mode. `repo` requires looking at subverb.
block contains {"reason": sprintf("helm %s blocked in read mode", [input.verb])} if {
    input.mode == "read"
    input.tool == "helm"
    input.verb != ""
    input.verb != "repo"
    not input.verb in read_verbs
}

block contains {"reason": sprintf("helm repo %s blocked in read mode", [input.subverb])} if {
    input.mode == "read"
    input.tool == "helm"
    input.verb == "repo"
    input.subverb != "list"
}
