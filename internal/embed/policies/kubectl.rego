package guard.bundled.kubectl

import future.keywords.if
import future.keywords.in
import future.keywords.contains

read_verbs := {
    "get", "describe", "logs", "exec", "port-forward", "top",
    "version", "cluster-info", "config", "explain",
    "api-resources", "api-versions", "auth", "diff", "wait",
}

# Block all non-read verbs in read mode, except --dry-run forms of `apply`.
block contains {"reason": sprintf("kubectl %s blocked in read mode", [input.verb])} if {
    input.mode == "read"
    input.tool == "kubectl"
    input.verb != ""
    not input.verb in read_verbs
    not allowed_dry_run
}

# Dry-run carve-out: if `apply` is invoked with --dry-run=client|server, allow.
allowed_dry_run if {
    input.verb == "apply"
    input.flags["dry-run"] in {"client", "server", "true"}
}
