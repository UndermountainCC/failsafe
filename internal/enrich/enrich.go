// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package enrich populates per-tool namespaces in the Rego fact (e.g.,
// fact.git.remote_url, fact.kubectl.cluster_name) with values that aren't
// derivable from argv alone. Enrichers are registered Go functions and
// must satisfy the §3.6 contract:
//   - pure of (parsed call, cwd)
//   - no subprocesses, no network
//   - 100ms timeout via the passed context.Context
//   - fail-with-partial-fact (panic/error/cancel → engine continues)
//
// Adding a new enricher requires a Go PR. The registry is curated.
package enrich

import (
	"context"
	"time"
)

// EnricherTimeout is the per-enricher budget. Spec §3.6 mandates 100ms.
const EnricherTimeout = 100 * time.Millisecond

// Enricher mutates a fact map in place under a deadline-bound context.
// Implementations should use ctx for any potentially blocking call and
// must be safe to interrupt (write nothing rather than write partial
// dangerous values).
type Enricher interface {
	Name() string
	Enrich(ctx context.Context, fact map[string]any)
}

// Registry holds enrichers by name.
type Registry struct{ byName map[string]Enricher }

func NewRegistry() *Registry { return &Registry{byName: map[string]Enricher{}} }

func (r *Registry) Register(e Enricher) { r.byName[e.Name()] = e }

func (r *Registry) Get(name string) (Enricher, bool) {
	e, ok := r.byName[name]
	return e, ok
}

// RunAll invokes each named enricher with a fresh 100ms-deadlined context.
// Panics inside an enricher are recovered; subsequent enrichers still run.
// Errors and timeouts are silent — the fact is whatever it is. The engine
// proceeds either way (fail-with-partial-fact, never fail-the-hook).
func (r *Registry) RunAll(names []string, fact map[string]any) {
	for _, name := range names {
		e, ok := r.byName[name]
		if !ok {
			continue
		}
		runOne(e, fact)
	}
}

func runOne(e Enricher, fact map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), EnricherTimeout)
	defer cancel()
	defer func() {
		// Swallow panics so a buggy enricher doesn't fail the hook.
		// Per §3.6 fail-with-partial-fact: the fact is what the enricher
		// managed to populate before panicking; we don't re-panic.
		_ = recover()
	}()
	e.Enrich(ctx, fact)
}
