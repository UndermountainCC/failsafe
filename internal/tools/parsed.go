// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

// Parsed is the output of a Tool's argv parser. It is the per-tool slice
// of the Rego fact: the engine's fact builder takes Parsed plus mode/cwd/
// enrichments and assembles the final Rego input.
type Parsed struct {
	Verb       string                 // first non-flag positional after the tool token
	Subverb    string                 // optional second-level (e.g., "list" for "terraform state list")
	Flags      map[string]interface{} // normalized: shorts collapsed to longs; bool flags = true; value flags = string
	Positional []string               // remaining tokens after flags
	Env        map[string]string      // KEY=VAL prefixes from the command
}
