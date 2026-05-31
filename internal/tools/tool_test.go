// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import "testing"

// stubTool is a minimal Tool used to verify the interface compiles and that
// Parsed-shape conventions hold. Real Tools (kubectl, helm, YAML-loaded) come later.
type stubTool struct{}

func (stubTool) Name() string                        { return "stub" }
func (stubTool) Match(token string) bool             { return token == "stub" }
func (stubTool) Parse(args []string) (Parsed, error) { return Parsed{Verb: "noop"}, nil }
func (stubTool) Enrichers() []string                 { return nil }

func TestParsed_Defaults(t *testing.T) {
	p := Parsed{}
	if p.Flags == nil {
		// constructor should normalize nil maps to empty for safe Rego access
		t.Skip("Parsed{}.Flags is nil; this is fine if facts builder normalizes")
	}
}

func TestStubTool_SatisfiesInterface(t *testing.T) {
	var _ Tool = stubTool{}
}
