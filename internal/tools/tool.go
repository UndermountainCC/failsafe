// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package tools defines the Tool interface and registry. Tools come from two
// sources: Go-coded (kubectl, helm — see kubectl.go and helm.go) and
// YAML-loaded (terraform, aws, git, and any user-added — see yamltool.go).
// The engine treats all sources symmetrically through this interface.
package tools

// Tool recognizes a shell binary and parses its argv into a Parsed result.
// Implementations must be pure: no globals, no I/O, deterministic.
type Tool interface {
	// Name is the unique tool identifier in the registry (e.g., "kubectl").
	Name() string

	// Match returns true if the given token (a single shell word) invokes
	// this tool. Implementations should accept "kubectl" and "/path/to/kubectl".
	Match(token string) bool

	// Parse takes the argv tail (everything after the tool token) and returns
	// a Parsed result. Returns an error only on truly malformed input;
	// missing-verb is represented as Parsed{Verb: ""}, not an error.
	Parse(args []string) (Parsed, error)

	// Enrichers names the enricher functions to run after parsing.
	// See internal/enrich/. Empty for tools that need no enrichment.
	Enrichers() []string
}
