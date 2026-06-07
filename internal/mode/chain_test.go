// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package mode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChain_FirstResolves(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "fallback"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fallback", "mode"), []byte("read & write"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FAILSAFE_TEST_X", "")
	chain := Chain{
		Sources: []Source{
			EnvSource{Name: "FAILSAFE_TEST_X"},
			FileSource{Pattern: "${HOME}/fallback/mode"},
		},
		Default: "read",
	}
	val, src, err := chain.Resolve(map[string]string{"HOME": dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "disabled" {
		t.Errorf("val = %q, want 'disabled' (canonicalized from 'read & write')", val)
	}
	if src == nil {
		t.Error("src should be non-nil for resolved value")
	}
}

func TestChain_AllSkipFallsToDefault(t *testing.T) {
	chain := Chain{
		Sources: []Source{
			EnvSource{Name: "FAILSAFE_TEST_NEVER_SET"},
			FileSource{Pattern: "${MISSING}/foo"},
		},
		Default: "read",
	}
	val, src, err := chain.Resolve(map[string]string{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "enabled" {
		t.Errorf("val = %q, want 'enabled' (canonicalized from legacy default 'read')", val)
	}
	if src != nil {
		t.Error("src should be nil when default is used")
	}
}

// TestChain_PerTTYBeatsGlobal: with both a per-tty file and a global file
// present, the per-tty value wins because it sits earlier in the chain.
func TestChain_PerTTYBeatsGlobal(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".config", "failsafe")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "tty-abc"), []byte("read & write"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "mode"), []byte("read"), 0o644); err != nil {
		t.Fatal(err)
	}
	chain := Chain{
		Sources: []Source{
			TTYSource{Dir: "${HOME}/.config/failsafe", TTY: func() (string, bool) { return "abc", true }},
			FileSource{Pattern: "${HOME}/.config/failsafe/mode"},
		},
		Default: "read",
	}
	val, _, err := chain.Resolve(map[string]string{"HOME": dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "disabled" {
		t.Errorf("val = %q, want per-tty value 'disabled' (canonicalized from 'read & write')", val)
	}
}

// TestChain_ColdPlainShellTogglesToPerTTY verifies that a plain shell with NO
// terminal-multiplexer vars set must still resolve a writable target — and it
// should be the isolated per-tty file, not the shared global, when a
// controlling tty exists.
func TestChain_ColdPlainShellTogglesToPerTTY(t *testing.T) {
	dir := t.TempDir()
	chain := Chain{
		Sources: []Source{
			EnvSource{Name: "FAILSAFE_MODE"},
			FileSource{Pattern: "${HOME}/.claude/pane-mode/${WEZTERM_PANE}"},
			FileSource{Pattern: "${HOME}/.claude/pane-mode/${TMUX_PANE}"},
			TTYSource{Dir: "${HOME}/.config/failsafe", TTY: func() (string, bool) { return "abc", true }},
			FileSource{Pattern: "${HOME}/.config/failsafe/mode"},
		},
		Default: "read",
	}
	// Plain shell: only HOME is set; no WEZTERM_PANE/TMUX_PANE/etc.
	env := map[string]string{"HOME": dir}
	_, path, ok := chain.FirstWritable(env)
	if !ok {
		t.Fatal("cold plain shell must still find a writable target")
	}
	if want := filepath.Join(dir, ".config", "failsafe", "tty-abc"); path != want {
		t.Errorf("writable target = %q, want per-tty %q", path, want)
	}
}

// TestChain_NoTTYFallsToGlobalWritable: headless/CI (no controlling tty, no
// terminal vars) still resolves a writable target — the global file.
func TestChain_NoTTYFallsToGlobalWritable(t *testing.T) {
	dir := t.TempDir()
	chain := Chain{
		Sources: []Source{
			TTYSource{Dir: "${HOME}/.config/failsafe", TTY: func() (string, bool) { return "", false }},
			FileSource{Pattern: "${HOME}/.config/failsafe/mode"},
		},
		Default: "read",
	}
	_, path, ok := chain.FirstWritable(map[string]string{"HOME": dir})
	if !ok {
		t.Fatal("expected global writable fallback when no tty")
	}
	if want := filepath.Join(dir, ".config", "failsafe", "mode"); path != want {
		t.Errorf("writable target = %q, want global %q", path, want)
	}
}

func TestResolveMigratesLegacyAndFailsSafe(t *testing.T) {
	cases := []struct{ in, want string }{
		{"enabled", "enabled"},
		{"disabled", "disabled"},
		{"read", "enabled"},
		{"read & write", "disabled"},
		{"", "enabled"},
		{"garbage", "enabled"},
	}
	for _, c := range cases {
		ch := Chain{Sources: []Source{EnvSource{Name: "M"}}, Default: "enabled"}
		got, _, err := ch.Resolve(map[string]string{"M": c.in})
		if err != nil || got != c.want {
			t.Fatalf("Resolve(%q) = %q,%v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestChain_FirstWritableForToggle(t *testing.T) {
	chain := Chain{
		Sources: []Source{
			EnvSource{Name: "FAILSAFE_X"}, // not writable
			FileSource{Pattern: "${HOME}/.failsafe/mode"},
		},
	}
	dir := t.TempDir()
	src, path, ok := chain.FirstWritable(map[string]string{"HOME": dir})
	if !ok {
		t.Fatal("expected writable source")
	}
	if src == nil {
		t.Error("src should be non-nil")
	}
	if path != filepath.Join(dir, ".failsafe", "mode") {
		t.Errorf("path = %q", path)
	}
}
