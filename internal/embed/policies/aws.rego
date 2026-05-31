package guard.bundled.aws

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# Note: bundled policies emit `block` only. `aws sts ...` and `aws s3 ls`
# are not blocked simply because no `block` rule below matches them.

# `aws s3 ls` allowed; other s3 verbs blocked. Skip when subverb is empty
# (e.g. `aws s3` with no action) — that's user error, not a mutation we
# need to gate.
block contains {"reason": sprintf("aws s3 %s blocked in read mode", [input.subverb])} if {
    input.mode == "read"
    input.tool == "aws"
    input.verb == "s3"
    input.subverb != ""
    input.subverb != "ls"
}

# For any other service, allow `describe-*`, `list-*`, `get-*`; block the rest.
# Empty subverb (e.g., `aws --help` or `aws ec2`) is also treated as harmless.
block contains {"reason": sprintf("aws %s %s blocked in read mode", [input.verb, input.subverb])} if {
    input.mode == "read"
    input.tool == "aws"
    input.verb != ""
    input.verb != "sts"
    input.verb != "s3"
    input.subverb != ""
    not is_read_action(input.subverb)
}

is_read_action(action) if startswith(action, "describe-")
is_read_action(action) if startswith(action, "list-")
is_read_action(action) if startswith(action, "get-")
