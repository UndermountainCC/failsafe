# Bundled policy for failsafe's own subcommands (dogfood: failsafe
# polices failsafe). All rules below fire regardless of mode — the
# read/read-and-write distinction is for kubectl/helm/etc. interactive
# mutations, not for the LLM toggling its own safety boundary.
package failsafe.bundled.failsafe

import future.keywords.if
import future.keywords.contains
import future.keywords.in

# `toggle` flips the safety boundary. The LLM must never invoke this; the
# user toggles from their own terminal (CLI or hotkey-bound iTerm/WezTerm
# coprocess), which is not on Claude's tool path.
block contains {"reason": "failsafe toggle is user-only — toggle from your terminal (hotkey or `failsafe toggle` typed yourself), never via Claude"} if {
    input.tool == "failsafe"
    input.verb == "toggle"
}

# `hook` and `mcp` are long-running lifecycle subprocesses started by
# Claude Code itself (the PreToolUse hook config and .mcp.json server
# entry). Claude making a tool call to invoke them directly is malformed —
# they expect specific stdin shapes from the parent runtime. Block as
# fail-fast diagnostic.
block contains {"reason": sprintf("failsafe %s is a lifecycle subprocess started by Claude Code; not for direct tool invocation", [input.verb])} if {
    input.tool == "failsafe"
    input.verb in {"hook", "mcp"}
}
