package failsafe.bundled.terraform

import future.keywords.if
import future.keywords.in
import future.keywords.contains

read_verbs := {"plan", "show", "output", "validate", "fmt", "providers", "version", "graph"}

block contains {"reason": sprintf("terraform %s blocked while failsafe is enabled", [input.verb])} if {
    not input.failsafe_enabled == false
    input.tool == "terraform"
    input.verb != ""
    input.verb != "state"
    not input.verb in read_verbs
}

block contains {"reason": sprintf("terraform state %s blocked while failsafe is enabled", [input.subverb])} if {
    not input.failsafe_enabled == false
    input.tool == "terraform"
    input.verb == "state"
    not input.subverb in {"list", "show"}
}
