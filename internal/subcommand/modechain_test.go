// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UndermountainCC/failsafe/internal/config"
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

// TestBuildModeChain_CustomPaneDir verifies that a HookOptions{Cfg: ...} with a
// custom Mode.PaneDir drives the mode file resolution under that directory,
// proving config actually wires through to the mode chain.
func TestBuildModeChain_CustomPaneDir(t *testing.T) {
	// Set up a fake HOME with a custom pane dir.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	customPaneDir := filepath.Join(home, "custom-pane-mode")
	if err := os.MkdirAll(customPaneDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write "read" into the custom pane dir for pane "99".
	if err := os.WriteFile(filepath.Join(customPaneDir, "99"), []byte("read"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build a config with the custom PaneDir.
	cfg := &config.Config{
		Mode: config.ModeConfig{
			PaneDir: customPaneDir,
		},
		Log: config.LogConfig{
			Enabled: false,
			Path:    filepath.Join(home, "decisions.jsonl"),
			Redact:  true,
		},
		Policy: config.PolicyConfig{
			UserPath: filepath.Join(home, ".config", "failsafe", "policy.rego"),
			ToolsDir: filepath.Join(home, ".config", "failsafe", "tools"),
		},
		Trust: config.TrustConfig{
			Path: filepath.Join(home, ".config", "failsafe", "trusted-repos.yaml"),
		},
	}

	// Verify buildModeChain produces a chain that resolves under customPaneDir.
	chain := buildModeChain(cfg, home)
	t.Setenv("WEZTERM_PANE", "99")
	env := map[string]string{"HOME": home, "WEZTERM_PANE": "99"}
	// "read" is the legacy vocabulary for "enabled"; Canonicalize maps it.
	val, src, err := chain.Resolve(env)
	if err != nil {
		t.Fatalf("chain.Resolve: %v", err)
	}
	if val != "enabled" {
		t.Errorf("expected canonicalized mode 'enabled' (from 'read' in pane file); got %q", val)
	}
	if src == nil {
		t.Error("expected a source (custom pane file) to have resolved; got nil (default used)")
	}

	// Confirm the chain patterns reference the custom pane dir, not the default.
	for _, src := range chain.Sources {
		if fs, ok := src.(mode.FileSource); ok {
			if strings.Contains(fs.Pattern, ".claude/pane-mode") {
				t.Errorf("chain should use custom pane dir %q but found default pattern %q",
					customPaneDir, fs.Pattern)
			}
		}
	}

	// Also verify that Hook uses the custom pane dir when Cfg is provided.
	t.Setenv("FAILSAFE_MODE", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("ITERM_SESSION_ID", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"kubectl apply -f foo.yaml"},"cwd":"/tmp","session_id":"sess-custom"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{
		Home: home,
		Cfg:  cfg,
	})
	if code != 0 {
		t.Fatalf("hook exit=%d stderr=%s", code, stderr.String())
	}
	// In "read" mode, kubectl apply should be blocked.
	if !strings.Contains(stdout.String(), `"decision":"block"`) {
		t.Errorf("expected block in read mode from custom pane dir; stdout=%s", stdout.String())
	}
}
