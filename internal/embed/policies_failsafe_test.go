// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package embed

// TestFailsafeGate_* verifies the fail-safe semantics of the bundled policies
// after the migration from input.mode == "read" to
// not input.failsafe_enabled == false.
//
// The critical property is fail-safe on missing field:
//   - field missing  → not undefined == false → not true → false → rule body
//     evaluates to true → BLOCK
//   - field=true     → not true == false → not false → true → BLOCK
//   - field=false    → not false == false → not false → true → wait, no…
//
// Correct truth table for `not input.failsafe_enabled == false`:
//   - missing  : (input.failsafe_enabled == false) is undefined → not undefined → true  → BLOCK ✓
//   - true     : (true == false) is false                       → not false     → true  → BLOCK ✓
//   - false    : (false == false) is true                       → not true      → false → condition fails → rule does NOT fire → ALLOW ✓

import (
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/rego"
)

// loadAndPrepare reads the named bundled policy, loads it alongside the
// given extra modules, and returns a PreparedEvalQuery for the given query.
func loadAndPrepare(t *testing.T, queryPkg, query string, mods map[string]string) rego.PreparedEvalQuery {
	t.Helper()
	ctx := context.Background()
	opts := []func(*rego.Rego){
		rego.Query(query),
	}
	for name, body := range mods {
		opts = append(opts, rego.Module(name, body))
	}
	pq, err := rego.New(opts...).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("PrepareForEval(%s): %v", query, err)
	}
	return pq
}

// evalBlock executes the prepared query against fact and returns all strings
// collected from the resulting set items' "reason" fields. An empty slice
// means no block rule fired (allow).
func evalBlock(t *testing.T, pq rego.PreparedEvalQuery, fact map[string]any) []string {
	t.Helper()
	rs, err := pq.Eval(context.Background(), rego.EvalInput(fact))
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	var reasons []string
	for _, r := range rs {
		for _, expr := range r.Expressions {
			items, ok := expr.Value.([]any)
			if !ok {
				continue
			}
			for _, item := range items {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if reason, ok := obj["reason"].(string); ok {
					reasons = append(reasons, reason)
				}
			}
		}
	}
	return reasons
}

// bundledModuleMap reads all bundled .rego files and returns them as
// map[filename]body so tests can load a consistent snapshot.
func bundledModuleMap(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, name := range BundledPolicyNames() {
		body, err := ReadBundledPolicy(name)
		if err != nil {
			t.Fatalf("ReadBundledPolicy(%s): %v", name, err)
		}
		out[name] = string(body)
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────────────
// kubectl gate

func kubectlPQ(t *testing.T) rego.PreparedEvalQuery {
	t.Helper()
	mods := bundledModuleMap(t)
	// determine the actual package name from kubectl.rego
	body := mods["kubectl.rego"]
	var pkg string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "package ") {
			pkg = strings.TrimPrefix(line, "package ")
			pkg = strings.TrimSpace(pkg)
			break
		}
	}
	if pkg == "" {
		t.Fatal("could not determine kubectl.rego package")
	}
	return loadAndPrepare(t, pkg, "data."+pkg+".block", mods)
}

// TestFailsafeGate_MissingField_Blocks proves the core fail-safe property:
// when the fact has NO failsafe_enabled key at all, a mutating kubectl command
// must still be BLOCKED (fail-safe / fail-closed on ambiguous input).
func TestFailsafeGate_MissingField_Blocks(t *testing.T) {
	pq := kubectlPQ(t)
	// Deliberately omit failsafe_enabled — simulates a misconfigured or
	// outdated fact builder that never sets the field.
	fact := map[string]any{
		"tool": "kubectl",
		"verb": "apply",
	}
	reasons := evalBlock(t, pq, fact)
	if len(reasons) == 0 {
		t.Fatal("FAIL-SAFE VIOLATION: kubectl apply with missing failsafe_enabled was ALLOWED — it must be BLOCKED")
	}
	t.Logf("correctly blocked with reason: %v", reasons)
}

// TestFailsafeGate_True_Blocks proves that failsafe_enabled=true also blocks.
func TestFailsafeGate_True_Blocks(t *testing.T) {
	pq := kubectlPQ(t)
	fact := map[string]any{
		"tool":             "kubectl",
		"verb":             "apply",
		"failsafe_enabled": true,
	}
	reasons := evalBlock(t, pq, fact)
	if len(reasons) == 0 {
		t.Fatal("kubectl apply with failsafe_enabled=true must be BLOCKED")
	}
	t.Logf("correctly blocked: %v", reasons)
}

// TestFailsafeGate_False_Allows proves that failsafe_enabled=false allows mutations.
func TestFailsafeGate_False_Allows(t *testing.T) {
	pq := kubectlPQ(t)
	fact := map[string]any{
		"tool":             "kubectl",
		"verb":             "apply",
		"failsafe_enabled": false,
	}
	reasons := evalBlock(t, pq, fact)
	if len(reasons) > 0 {
		t.Fatalf("kubectl apply with failsafe_enabled=false must be ALLOWED, got blocked: %v", reasons)
	}
	t.Log("correctly allowed")
}

// TestFailsafeGate_ReadVerb_AlwaysAllowed proves a read verb is never blocked
// regardless of failsafe_enabled.
func TestFailsafeGate_ReadVerb_AlwaysAllowed(t *testing.T) {
	pq := kubectlPQ(t)
	for _, fe := range []any{true, false, nil} {
		fact := map[string]any{
			"tool": "kubectl",
			"verb": "get",
		}
		if fe != nil {
			fact["failsafe_enabled"] = fe
		}
		reasons := evalBlock(t, pq, fact)
		if len(reasons) > 0 {
			t.Errorf("kubectl get with failsafe_enabled=%v must be ALLOWED, got blocked: %v", fe, reasons)
		}
	}
}
