// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"testing"

	"github.com/UndermountainCC/failsafe/internal/mode"
)

// TestDefaultModeChain_PerTTYBeforeGlobal pins the chain wiring: a per-tty
// source must sit immediately before the global ~/.config fallback so plain
// shells get an isolated (but still writable) target.
func TestDefaultModeChain_PerTTYBeforeGlobal(t *testing.T) {
	chain := defaultModeChain()
	globalIdx, ttyIdx := -1, -1
	for i, s := range chain.Sources {
		switch src := s.(type) {
		case mode.TTYSource:
			ttyIdx = i
		case mode.FileSource:
			if src.Pattern == "${HOME}/.config/failsafe/mode" {
				globalIdx = i
			}
		}
	}
	if globalIdx < 0 {
		t.Fatal("global fallback should be ${HOME}/.config/failsafe/mode")
	}
	if ttyIdx < 0 {
		t.Fatal("expected a per-tty source in the default chain")
	}
	if ttyIdx > globalIdx {
		t.Errorf("per-tty source (idx %d) must precede the global fallback (idx %d)", ttyIdx, globalIdx)
	}
}

func TestDefaultModeChainDefaultsToEnabled(t *testing.T) {
	ch := DefaultModeChain()
	got, src, _ := ch.Resolve(map[string]string{"HOME": t.TempDir()})
	if got != "enabled" || src != nil {
		t.Fatalf("default resolve = %q (src %v); want enabled/nil", got, src)
	}
}

// TestResolvePaneID_SkipsGlobalFallback guards the mcp pane-id display: the
// global config file resolves in any shell with HOME set, but its trailing
// component ("mode") must never be surfaced as a pane identifier.
func TestResolvePaneID_SkipsGlobalFallback(t *testing.T) {
	got := resolvePaneID(defaultModeChain(), map[string]string{"HOME": t.TempDir()})
	if got == "mode" {
		t.Errorf("global fallback leaked as pane id %q; it must be skipped", got)
	}
}
