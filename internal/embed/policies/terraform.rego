package guard.bundled.terraform

import future.keywords.if
import future.keywords.in
import future.keywords.contains

read_verbs := {"plan", "show", "output", "validate", "fmt", "providers", "version", "graph"}

block contains {"reason": sprintf("terraform %s blocked in read mode", [input.verb])} if {
    input.mode == "read"
    input.tool == "terraform"
    input.verb != ""
    input.verb != "state"
    not input.verb in read_verbs
}

block contains {"reason": sprintf("terraform state %s blocked in read mode", [input.subverb])} if {
    input.mode == "read"
    input.tool == "terraform"
    input.verb == "state"
    not input.subverb in {"list", "show"}
}
