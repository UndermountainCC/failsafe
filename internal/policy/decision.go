// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package policy is the OPA-backed policy evaluator. Inputs are facts (from
// internal/facts) plus a policy chain (bundled + user + repo). Outputs are
// Decisions: block-with-reason, allow, or allow-with-override-reason.
package policy

// Layer identifies the policy source.
type Layer string

const (
	LayerBundled Layer = "bundled"
	LayerUser    Layer = "user"
	LayerRepo    Layer = "repo"
)

// Decision is the engine output. Trace lists every rule firing for
// `failsafe explain`. The closest-to-cwd-then-layer order is applied
// before Trace is returned, so Trace[0] is the rule whose reason was
// surfaced.
type Decision struct {
	Block          bool      // true → Claude Code blocks the command
	Reason         string    // primary reason on block
	OverrideReason string    // populated when an allow_override matched
	Trace          []RuleHit // ordered: closest-to-cwd first
}

// RuleHit records a single rule firing.
//
// Malformed=true means the rule emitted output that wasn't a proper
// {"reason": <string>} object; the engine substituted a synthetic reason
// describing the bug. For block rules, malformed hits still BLOCK (fail
// closed: a buggy block rule still blocks the action). For allow_override
// rules, malformed hits do NOT unblock — the engine filters them out
// before deciding (fail closed: a buggy override doesn't free a real
// block). The Trace still includes malformed hits so audit/explain can
// show the bug.
type RuleHit struct {
	Layer     Layer
	File      string
	Line      int
	Name      string // "block" or "allow_override"
	Reason    string
	Malformed bool
}
