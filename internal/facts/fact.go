// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package facts assembles the Rego input fact from a parsed command,
// mode-source result, hook context, and enrichers.
package facts

import (
	"time"

	"github.com/UndermountainCC/failsafe/internal/enrich"
	"github.com/UndermountainCC/failsafe/internal/tools"
)

// Builder collects everything the Build step needs.
//
// Enrichers are invoked through Enrichers.RunAll so the §3.6 contract
// (100ms timeout per enricher, panic recovery, fail-with-partial-fact) is
// honored. The builder does NOT call enrichers directly — that bypasses
// the safety boundary.
type Builder struct {
	Mode      string
	Tool      string
	CWD       string
	Now       time.Time
	SessionID string
	Pane      string

	Parsed tools.Parsed
	Env    map[string]string
	Raw    string

	// EnricherNames lists which enrichers in Enrichers to invoke.
	// Typically: Tool.Enrichers() (e.g., ["git"] or ["kubectl_context"]).
	EnricherNames []string
	// Enrichers is the registry containing all available enricher
	// implementations. Pass nil to skip enrichment (useful in tests).
	Enrichers *enrich.Registry
}

// Build returns the fact as a Rego-friendly map[string]any.
func (b Builder) Build() map[string]any {
	flags := map[string]any{}
	for k, v := range b.Parsed.Flags {
		flags[k] = v
	}
	env := map[string]any{}
	for k, v := range b.Env {
		env[k] = v
	}
	fact := map[string]any{
		"mode":       b.Mode,
		"tool":       b.Tool,
		"verb":       b.Parsed.Verb,
		"subverb":    b.Parsed.Subverb,
		"flags":      flags,
		"positional": stringsToAny(b.Parsed.Positional),
		"env":        env,
		"cwd":        b.CWD,
		"now":        b.Now.UTC().Format(time.RFC3339),
		"session": map[string]any{
			"claude_session_id": b.SessionID,
			"wezterm_pane":      b.Pane,
		},
		"raw": b.Raw,
	}
	if b.Enrichers != nil && len(b.EnricherNames) > 0 {
		b.Enrichers.RunAll(b.EnricherNames, fact)
	}
	return fact
}

func stringsToAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}
