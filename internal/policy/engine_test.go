// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"strings"
	"testing"
)

const allowOnlyRego = `package guard.bundled.x
import future.keywords.if
# no rules — should result in allow
`

const blockKubectlApply = `package guard.bundled.k
import future.keywords.if
import future.keywords.contains

block contains {"reason": "kubectl apply blocked (bundled)"} if {
    input.tool == "kubectl"
    input.verb == "apply"
}
`

const repoOverride = `package guard.repo
import future.keywords.if
import future.keywords.contains

allow_override contains {"reason": "test override"} if { true }
`

const userTriesOverride = `package guard.user
import future.keywords.if
import future.keywords.contains

allow_override contains {"reason": "illegal user override"} if { true }
`

const repoBlockExtra = `package guard.repo
import future.keywords.if
import future.keywords.contains

block contains {"reason": "repo extra deny"} if {
    input.tool == "kubectl"
    input.verb == "apply"
}
`

func TestEngine_NoRulesAllows(t *testing.T) {
	e, err := New("/tmp/cwd", []Module{{Layer: LayerBundled, File: "x.rego", Body: allowOnlyRego}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	d, err := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Block {
		t.Errorf("expected allow, got block reason=%q", d.Reason)
	}
}

func TestEngine_BlockRuleBlocks(t *testing.T) {
	e, err := New("/tmp/cwd", []Module{{Layer: LayerBundled, File: "k.rego", Body: blockKubectlApply}})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl", "verb": "apply"})
	if !d.Block {
		t.Error("expected block")
	}
	if !strings.Contains(d.Reason, "kubectl apply blocked") {
		t.Errorf("reason = %q", d.Reason)
	}
	if len(d.Trace) != 1 {
		t.Errorf("trace length = %d, want 1", len(d.Trace))
	}
	if d.Trace[0].Layer != LayerBundled || d.Trace[0].File != "k.rego" {
		t.Errorf("trace[0] = %+v", d.Trace[0])
	}
}

func TestEngine_TrustedRepoOverrideUnblocks(t *testing.T) {
	e, err := New("/Users/you/Code/x", []Module{
		{Layer: LayerBundled, File: "k.rego", Body: blockKubectlApply},
		{Layer: LayerRepo, File: "/Users/you/Code/x/.failsafe.rego", Body: repoOverride, Trusted: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl", "verb": "apply"})
	if d.Block {
		t.Errorf("expected allow due to override, got block: %q", d.Reason)
	}
	if d.OverrideReason != "test override" {
		t.Errorf("OverrideReason = %q", d.OverrideReason)
	}
}

func TestEngine_UntrustedRepoOverrideDropped(t *testing.T) {
	// Same modules as above, but Trusted=false on the repo. allow_override
	// must be filtered at compile, so the bundled block fires unopposed.
	e, err := New("/Users/you/Code/x", []Module{
		{Layer: LayerBundled, File: "k.rego", Body: blockKubectlApply},
		{Layer: LayerRepo, File: "/Users/you/Code/x/.failsafe.rego", Body: repoOverride, Trusted: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl", "verb": "apply"})
	if !d.Block {
		t.Error("expected block; untrusted-repo override should be dropped")
	}
	if d.OverrideReason != "" {
		t.Errorf("expected no override reason, got %q", d.OverrideReason)
	}
}

func TestEngine_UserCannotEmitOverride(t *testing.T) {
	_, err := New("/tmp", []Module{{Layer: LayerUser, File: "policy.rego", Body: userTriesOverride}})
	if err == nil {
		t.Fatal("expected New to reject allow_override in user layer")
	}
	if !strings.Contains(err.Error(), "allow_override") {
		t.Errorf("error should mention allow_override: %v", err)
	}
}

// Malformed allow_override must NOT unblock a real block (fail-closed for
// overrides). The buggy override hit appears in the trace but doesn't
// influence the decision.
func TestEngine_MalformedOverrideDoesNotUnblock(t *testing.T) {
	const malformedOverride = `package guard.repo
import future.keywords.if
import future.keywords.contains

# Malformed: missing "reason" key.
allow_override contains {"why": "broken"} if { true }
`
	e, err := New("/repo", []Module{
		{Layer: LayerBundled, File: "k.rego", Body: blockKubectlApply},
		{Layer: LayerRepo, File: "/repo/.failsafe.rego", Body: malformedOverride, Trusted: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl", "verb": "apply"})
	if !d.Block {
		t.Fatalf("expected block — malformed override must not unblock; got allow with override=%q", d.OverrideReason)
	}
	// The trace should mention the malformed override for visibility.
	sawMalformed := false
	for _, h := range d.Trace {
		if h.Malformed {
			sawMalformed = true
			break
		}
	}
	if !sawMalformed {
		t.Errorf("expected trace to include the malformed override hit; trace=%+v", d.Trace)
	}
}

// Malformed block STILL blocks (fail-closed for blocks). The user sees a
// "policy bug ..." reason rather than a silent allow.
func TestEngine_MalformedBlockStillBlocks(t *testing.T) {
	const malformedBlock = `package guard.bundled.x
import future.keywords.if
import future.keywords.contains

# Wrong shape: integer reason instead of string.
block contains {"reason": 42} if { true }
`
	e, err := New("/cwd", []Module{
		{Layer: LayerBundled, File: "x.rego", Body: malformedBlock},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl"})
	if !d.Block {
		t.Fatal("expected block (fail-closed) for malformed block rule")
	}
	if !strings.Contains(d.Reason, "policy bug") {
		t.Errorf("expected policy-bug reason, got %q", d.Reason)
	}
}

// A malformed block rule MUST block even when a valid override matches.
// Otherwise a buggy block + a working override silently allows the action,
// which is fail-open. The malformed-block reason takes precedence over the
// override so the policy bug surfaces to the user.
func TestEngine_MalformedBlockBlocksEvenWithValidOverride(t *testing.T) {
	const malformedBlock = `package guard.bundled.x
import future.keywords.if
import future.keywords.contains

block contains {"reason": 42} if { true }
`
	const validRepoOverride = `package guard.repo
import future.keywords.if
import future.keywords.contains

allow_override contains {"reason": "this should NOT mask the policy bug"} if { true }
`
	e, err := New("/repo", []Module{
		{Layer: LayerBundled, File: "x.rego", Body: malformedBlock},
		{Layer: LayerRepo, File: "/repo/.failsafe.rego", Body: validRepoOverride, Trusted: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl"})
	if !d.Block {
		t.Fatalf("expected block — malformed block rule must NOT be unblocked by override; got allow with override=%q", d.OverrideReason)
	}
	if !strings.Contains(d.Reason, "policy bug") {
		t.Errorf("expected policy-bug reason; got %q", d.Reason)
	}
}

func TestEngine_ReasonPrecedenceClosestToCWDFirst(t *testing.T) {
	cwd := "/Users/you/Code/monorepo/sub"
	e, err := New(cwd, []Module{
		{Layer: LayerBundled, File: "bundled/k.rego", Body: blockKubectlApply},
		{Layer: LayerRepo, File: "/Users/you/Code/monorepo/.failsafe.rego", Body: repoBlockExtra, Trusted: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate(context.Background(), map[string]any{"tool": "kubectl", "verb": "apply"})
	if !d.Block {
		t.Fatal("expected block")
	}
	// Closest-to-cwd repo file beats bundled.
	if !strings.Contains(d.Reason, "repo extra deny") {
		t.Errorf("reason = %q; expected repo extra deny first (closer to cwd)", d.Reason)
	}
	// Both rules should appear in the trace.
	if len(d.Trace) != 2 {
		t.Errorf("trace length = %d, want 2 (got %+v)", len(d.Trace), d.Trace)
	}
}
