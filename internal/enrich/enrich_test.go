// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package enrich

import (
	"context"
	"testing"
	"time"
)

type sleepEnricher struct {
	name string
	dur  time.Duration
}

func (s sleepEnricher) Name() string { return s.name }
func (s sleepEnricher) Enrich(ctx context.Context, fact map[string]any) {
	select {
	case <-time.After(s.dur):
		fact[s.name] = "ran"
	case <-ctx.Done():
		fact[s.name] = "cancelled"
	}
}

type panicEnricher struct{}

func (panicEnricher) Name() string                               { return "panic" }
func (panicEnricher) Enrich(_ context.Context, _ map[string]any) { panic("boom") }

func TestRunAll_FastEnricherCompletes(t *testing.T) {
	r := NewRegistry()
	r.Register(sleepEnricher{name: "fast", dur: 5 * time.Millisecond})
	fact := map[string]any{}
	r.RunAll([]string{"fast"}, fact)
	if fact["fast"] != "ran" {
		t.Errorf("fast = %v, want ran", fact["fast"])
	}
}

func TestRunAll_SlowEnricherIsCancelled(t *testing.T) {
	r := NewRegistry()
	r.Register(sleepEnricher{name: "slow", dur: 500 * time.Millisecond})
	fact := map[string]any{}
	start := time.Now()
	r.RunAll([]string{"slow"}, fact)
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("RunAll took %v; should cap at ~100ms", elapsed)
	}
	// fact may be empty (cancel arrived before the select branch executed)
	// or contain "cancelled"; both are acceptable per fail-with-partial.
}

func TestRunAll_PanicSwallowed(t *testing.T) {
	r := NewRegistry()
	r.Register(panicEnricher{})
	r.Register(sleepEnricher{name: "after", dur: 1 * time.Millisecond})
	fact := map[string]any{}
	// Must not propagate the panic.
	r.RunAll([]string{"panic", "after"}, fact)
	if fact["after"] != "ran" {
		t.Errorf("subsequent enricher should run despite earlier panic; fact = %v", fact)
	}
}

func TestRunAll_UnknownNameIgnored(t *testing.T) {
	r := NewRegistry()
	fact := map[string]any{}
	r.RunAll([]string{"nope"}, fact) // must not panic, must not error
}
