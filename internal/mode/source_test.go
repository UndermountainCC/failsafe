// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package mode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvSource_Resolve(t *testing.T) {
	t.Setenv("FAILSAFE_TEST_MODE", "read & write")
	src := EnvSource{Name: "FAILSAFE_TEST_MODE"}
	val, ok, err := src.Resolve(envOnly())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "read & write" {
		t.Errorf("val = %q, want 'read & write'", val)
	}
}

func TestEnvSource_Unset(t *testing.T) {
	src := EnvSource{Name: "DEFINITELY_NOT_SET_FAILSAFE_TEST"}
	_, ok, _ := src.Resolve(envOnly())
	if ok {
		t.Error("expected ok=false for unset env var")
	}
}

func TestFileSource_VarMissingSkips(t *testing.T) {
	src := FileSource{Pattern: "${HOME}/.claude/pane-mode/${WEZTERM_PANE}"}
	// WEZTERM_PANE not set in test env
	t.Setenv("WEZTERM_PANE", "")
	val, ok, err := src.Resolve(map[string]string{"HOME": t.TempDir()})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ok {
		t.Errorf("expected source to be skipped when var unset; got val=%q", val)
	}
}

func TestFileSource_Resolves(t *testing.T) {
	dir := t.TempDir()
	paneDir := filepath.Join(dir, ".claude", "pane-mode")
	if err := os.MkdirAll(paneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paneDir, "42"), []byte("read & write\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := FileSource{Pattern: "${HOME}/.claude/pane-mode/${PANE}"}
	val, ok, err := src.Resolve(map[string]string{"HOME": dir, "PANE": "42"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "read & write" {
		t.Errorf("val = %q, want 'read & write' (trimmed)", val)
	}
}

func TestFileSource_MissingFile(t *testing.T) {
	dir := t.TempDir()
	src := FileSource{Pattern: "${HOME}/.claude/pane-mode/${PANE}"}
	_, ok, err := src.Resolve(map[string]string{"HOME": dir, "PANE": "404"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ok {
		t.Error("expected ok=false when file missing")
	}
}

func TestTTYSource_ResolvesPerTTYFile(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".config", "failsafe")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "tty-dev123"), []byte("read & write\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := TTYSource{Dir: "${HOME}/.config/failsafe", TTY: func() (string, bool) { return "dev123", true }}
	val, ok, err := src.Resolve(map[string]string{"HOME": dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for an existing per-tty file")
	}
	if val != "read & write" {
		t.Errorf("val = %q, want 'read & write'", val)
	}
}

func TestTTYSource_SkipsWhenNoTTY(t *testing.T) {
	src := TTYSource{Dir: "${HOME}/.config/failsafe", TTY: func() (string, bool) { return "", false }}
	_, ok, err := src.Resolve(map[string]string{"HOME": t.TempDir()})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ok {
		t.Error("expected source skipped when no controlling tty (e.g. headless/CI)")
	}
	if _, pok := src.Path(map[string]string{"HOME": "/x"}); pok {
		t.Error("Path should be unresolved when there is no controlling tty")
	}
}

func TestTTYSource_WritablePath(t *testing.T) {
	src := TTYSource{Dir: "${HOME}/.config/failsafe", TTY: func() (string, bool) { return "dev123", true }}
	if !src.Writable() {
		t.Error("per-tty source must be writable so a plain shell can toggle")
	}
	p, ok := src.Path(map[string]string{"HOME": "/home/u"})
	if !ok {
		t.Fatal("expected path to resolve when tty + HOME are known")
	}
	if want := "/home/u/.config/failsafe/tty-dev123"; p != want {
		t.Errorf("path = %q, want %q", p, want)
	}
}

// envOnly returns a map of just the OS env so that Source tests run in isolation.
func envOnly() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				out[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return out
}
