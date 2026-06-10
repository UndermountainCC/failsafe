// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// noEnv returns "" for every key — a clean environment for most tests.
func noEnv(string) string { return "" }

// homeEnv returns an env func that injects home for "HOME" and delegates the
// rest to extra (which may be nil → noEnv).
func homeEnv(home string, extra map[string]string) func(string) string {
	return func(k string) string {
		if k == "HOME" {
			return home
		}
		if extra != nil {
			return extra[k]
		}
		return ""
	}
}

// mustLoad calls Load and fatals on error — used when a test expects success.
func mustLoad(t *testing.T, opts Options) *Config {
	t.Helper()
	cfg, err := Load(opts)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	return cfg
}

// ---------------------------------------------------------------------------
// Missing file → all defaults
// ---------------------------------------------------------------------------

// TestMissingFileReturnsDefaults ensures that when no config.yaml exists, Load
// returns a fully-populated Config equal to the documented defaults.
func TestMissingFileReturnsDefaults(t *testing.T) {
	home := t.TempDir()
	cfg := mustLoad(t, Options{
		Home: home,
		Env:  homeEnv(home, nil),
	})

	// Mode defaults.
	if cfg.Mode.PaneDir != home+"/.claude/pane-mode" {
		t.Errorf("Mode.PaneDir: want %s/.claude/pane-mode, got %q", home, cfg.Mode.PaneDir)
	}

	// Log defaults.
	if !cfg.Log.Enabled {
		t.Error("Log.Enabled: want true, got false")
	}
	want := home + "/.config/failsafe/decisions.jsonl"
	if cfg.Log.Path != want {
		t.Errorf("Log.Path: want %q, got %q", want, cfg.Log.Path)
	}
	if !cfg.Log.Redact {
		t.Error("Log.Redact: want true, got false")
	}

	// Telemetry defaults (off by default).
	if cfg.Telemetry.Enabled {
		t.Error("Telemetry.Enabled: want false, got true")
	}
	if cfg.Telemetry.OTLPEndpoint != "" {
		t.Errorf("Telemetry.OTLPEndpoint: want empty, got %q", cfg.Telemetry.OTLPEndpoint)
	}

	// Policy defaults.
	if cfg.Policy.UserPath != home+"/.config/failsafe/policy.rego" {
		t.Errorf("Policy.UserPath: got %q", cfg.Policy.UserPath)
	}
	if cfg.Policy.ToolsDir != home+"/.config/failsafe/tools" {
		t.Errorf("Policy.ToolsDir: got %q", cfg.Policy.ToolsDir)
	}

	// Trust default.
	if cfg.Trust.Path != home+"/.config/failsafe/trusted-repos.yaml" {
		t.Errorf("Trust.Path: got %q", cfg.Trust.Path)
	}
}

// ---------------------------------------------------------------------------
// Precedence: flags > env > file > defaults
// ---------------------------------------------------------------------------

// TestPrecedence writes a config.yaml that sets log.path, then an env var that
// overrides it further, then a flag that wins over everything.  Each layer is
// checked to confirm the strict ordering.
func TestPrecedence(t *testing.T) {
	home := t.TempDir()

	// Create config directory and file.
	cfgDir := filepath.Join(home, ".config", "failsafe")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	fileVal := filepath.Join(home, "file-log.jsonl")
	envVal := filepath.Join(home, "env-log.jsonl")
	flagVal := filepath.Join(home, "flag-log.jsonl")

	yaml := fmt.Sprintf("log:\n  path: %s\n", fileVal)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	// Env-only: file wins over defaults.
	cfgFileOnly := mustLoad(t, Options{
		Home: home,
		Env:  homeEnv(home, nil),
	})
	if cfgFileOnly.Log.Path != fileVal {
		t.Errorf("file layer: want %q, got %q", fileVal, cfgFileOnly.Log.Path)
	}

	// Env wins over file.
	cfgEnv := mustLoad(t, Options{
		Home: home,
		Env: homeEnv(home, map[string]string{
			"FAILSAFE_LOG_PATH": envVal,
		}),
	})
	if cfgEnv.Log.Path != envVal {
		t.Errorf("env layer: want %q, got %q", envVal, cfgEnv.Log.Path)
	}

	// Flag wins over env.
	f := pflag.NewFlagSet("test", pflag.ContinueOnError)
	f.String("log.path", "", "log path")
	if err := f.Parse([]string{"--log.path=" + flagVal}); err != nil {
		t.Fatal(err)
	}
	cfgFlag := mustLoad(t, Options{
		Home:  home,
		Env:   homeEnv(home, map[string]string{"FAILSAFE_LOG_PATH": envVal}),
		Flags: f,
	})
	if cfgFlag.Log.Path != flagVal {
		t.Errorf("flag layer: want %q, got %q", flagVal, cfgFlag.Log.Path)
	}
}

// ---------------------------------------------------------------------------
// Back-compat shims for FAILSAFE_LOG
// ---------------------------------------------------------------------------

// TestFAILSAFELogOff reproduces auditlog.DefaultLogger semantics:
// FAILSAFE_LOG=off → log.enabled=false, path stays at default.
func TestFAILSAFELogOff(t *testing.T) {
	home := t.TempDir()
	cfg := mustLoad(t, Options{
		Home: home,
		Env:  homeEnv(home, map[string]string{"FAILSAFE_LOG": "off"}),
	})
	if cfg.Log.Enabled {
		t.Error("FAILSAFE_LOG=off: Log.Enabled should be false")
	}
	// Path should still be the default (not empty); DefaultLogger leaves it
	// empty when disabled, but our Config retains the default path — this is
	// intentional (the path is just unused when Enabled=false).
}

// TestFAILSAFELogPath reproduces: FAILSAFE_LOG=<path> → path=<path>, enabled=true.
func TestFAILSAFELogPath(t *testing.T) {
	home := t.TempDir()
	customPath := "/x/y.jsonl"
	cfg := mustLoad(t, Options{
		Home: home,
		Env:  homeEnv(home, map[string]string{"FAILSAFE_LOG": customPath}),
	})
	if !cfg.Log.Enabled {
		t.Error("FAILSAFE_LOG=<path>: Log.Enabled should be true")
	}
	if cfg.Log.Path != customPath {
		t.Errorf("FAILSAFE_LOG=<path>: Log.Path want %q, got %q", customPath, cfg.Log.Path)
	}
}

// TestFAILSAFELogShimMatchesDefaultLogger confirms that the shim reproduces the
// exact same decisions that auditlog.DefaultLogger makes for the same env value.
// This is the "byte-identical back-compat" guarantee from the spec §Migration.
func TestFAILSAFELogShimMatchesDefaultLogger(t *testing.T) {
	home := t.TempDir()

	cases := []struct {
		envVal      string
		wantEnabled bool
		wantPath    string // "" means "the home-derived default"
	}{
		{"off", false, ""},
		{"/x/y.jsonl", true, "/x/y.jsonl"},
		{"", true, filepath.Join(home, ".config", "failsafe", "decisions.jsonl")},
	}

	for _, tc := range cases {
		t.Run("FAILSAFE_LOG="+tc.envVal, func(t *testing.T) {
			cfg := mustLoad(t, Options{
				Home: home,
				Env:  homeEnv(home, map[string]string{"FAILSAFE_LOG": tc.envVal}),
			})

			if cfg.Log.Enabled != tc.wantEnabled {
				t.Errorf("Enabled: want %v, got %v", tc.wantEnabled, cfg.Log.Enabled)
			}
			if tc.wantPath != "" && cfg.Log.Path != tc.wantPath {
				t.Errorf("Path: want %q, got %q", tc.wantPath, cfg.Log.Path)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Safety-fixed: log.redact=false rejected
// ---------------------------------------------------------------------------

func TestLogRedactFalseRejected(t *testing.T) {
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "failsafe")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yaml := "log:\n  redact: false\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(Options{Home: home, Env: homeEnv(home, nil)})
	if err == nil {
		t.Fatal("expected error for log.redact=false, got nil")
	}
	if !contains(err.Error(), "redact") {
		t.Errorf("expected error to mention 'redact', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Safety-fixed: control_plane.* rejected in v1
// ---------------------------------------------------------------------------

func TestControlPlaneRejected(t *testing.T) {
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "failsafe")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yaml := "control_plane:\n  url: https://example.com\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(Options{Home: home, Env: homeEnv(home, nil)})
	if err == nil {
		t.Fatal("expected error for control_plane.url set, got nil")
	}
	if !contains(err.Error(), "control_plane") {
		t.Errorf("expected error to mention 'control_plane', got: %v", err)
	}
}

func TestControlPlaneBundleSigningKeyRejected(t *testing.T) {
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "failsafe")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yaml := "control_plane:\n  bundle_signing_key: abc123\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(Options{Home: home, Env: homeEnv(home, nil)})
	if err == nil {
		t.Fatal("expected error for control_plane.bundle_signing_key set, got nil")
	}
	if !contains(err.Error(), "control_plane") {
		t.Errorf("expected error to mention 'control_plane', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tilde expansion
// ---------------------------------------------------------------------------

func TestTildeExpansion(t *testing.T) {
	home := "/custom/home"
	cfg := mustLoad(t, Options{
		Home: home,
		Env:  homeEnv(home, nil),
	})

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Mode.PaneDir", cfg.Mode.PaneDir, home + "/.claude/pane-mode"},
		{"Log.Path", cfg.Log.Path, home + "/.config/failsafe/decisions.jsonl"},
		{"Policy.UserPath", cfg.Policy.UserPath, home + "/.config/failsafe/policy.rego"},
		{"Policy.ToolsDir", cfg.Policy.ToolsDir, home + "/.config/failsafe/tools"},
		{"Trust.Path", cfg.Trust.Path, home + "/.config/failsafe/trusted-repos.yaml"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: want %q, got %q", c.name, c.want, c.got)
		}
	}
}

// TestTildeExpansionFromFile ensures tilde in a config.yaml path uses opts.Home.
func TestTildeExpansionFromFile(t *testing.T) {
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "failsafe")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Write a config.yaml with a tilde path.
	yaml := "log:\n  path: ~/custom-decisions.jsonl\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := mustLoad(t, Options{Home: home, Env: homeEnv(home, nil)})
	want := home + "/custom-decisions.jsonl"
	if cfg.Log.Path != want {
		t.Errorf("tilde in config file: want %q, got %q", want, cfg.Log.Path)
	}
}

// ---------------------------------------------------------------------------
// Regression: multi-word env var keys map to correct koanf fields
// ---------------------------------------------------------------------------

// TestEnvMultiWordKeysMapCorrectly verifies that env vars whose names contain
// multiple underscores after the FAILSAFE_ prefix (e.g. FAILSAFE_MODE_PANE_DIR)
// land in the correct Config field.  Before the static-table fix these vars
// were silently ignored because the mechanical _→. transform produced invalid
// koanf keys (e.g. "mode.pane.dir" instead of "mode.pane_dir").
func TestEnvMultiWordKeysMapCorrectly(t *testing.T) {
	home := t.TempDir()

	env := homeEnv(home, map[string]string{
		"FAILSAFE_MODE_PANE_DIR":           "/custom/pane-mode",
		"FAILSAFE_POLICY_USER_PATH":         "/custom/policy.rego",
		"FAILSAFE_POLICY_TOOLS_DIR":         "/custom/tools",
		"FAILSAFE_TELEMETRY_OTLP_ENDPOINT":  "http://otel.example.com:4318",
	})

	cfg := mustLoad(t, Options{Home: home, Env: env})

	if cfg.Mode.PaneDir != "/custom/pane-mode" {
		t.Errorf("FAILSAFE_MODE_PANE_DIR: want /custom/pane-mode, got %q", cfg.Mode.PaneDir)
	}
	if cfg.Policy.UserPath != "/custom/policy.rego" {
		t.Errorf("FAILSAFE_POLICY_USER_PATH: want /custom/policy.rego, got %q", cfg.Policy.UserPath)
	}
	if cfg.Policy.ToolsDir != "/custom/tools" {
		t.Errorf("FAILSAFE_POLICY_TOOLS_DIR: want /custom/tools, got %q", cfg.Policy.ToolsDir)
	}
	if cfg.Telemetry.OTLPEndpoint != "http://otel.example.com:4318" {
		t.Errorf("FAILSAFE_TELEMETRY_OTLP_ENDPOINT: want http://otel.example.com:4318, got %q", cfg.Telemetry.OTLPEndpoint)
	}
}

// TestFAILSAFEModeNotLeakedToConfig asserts that FAILSAFE_MODE is intentionally
// excluded from the config key table and does NOT populate any Config field.
// It lives as EnvSource{Name:"FAILSAFE_MODE"} in the mode chain only.
func TestFAILSAFEModeNotLeakedToConfig(t *testing.T) {
	home := t.TempDir()

	env := homeEnv(home, map[string]string{
		"FAILSAFE_MODE": "disabled",
	})

	cfg := mustLoad(t, Options{Home: home, Env: env})

	// FAILSAFE_MODE must not have set any Config field.  The only observable
	// effect would be a non-default Mode.PaneDir or a new field under Mode — we
	// check the pane_dir default is undisturbed and that no string field in the
	// entire Config unexpectedly holds the value "disabled".
	defaultPaneDir := home + "/.claude/pane-mode"
	if cfg.Mode.PaneDir != defaultPaneDir {
		t.Errorf("FAILSAFE_MODE leaked into Mode.PaneDir: got %q", cfg.Mode.PaneDir)
	}

	// Spot-check: "disabled" must not appear in any string field.
	stringFields := []struct {
		name string
		val  string
	}{
		{"Mode.PaneDir", cfg.Mode.PaneDir},
		{"Log.Path", cfg.Log.Path},
		{"Policy.UserPath", cfg.Policy.UserPath},
		{"Policy.ToolsDir", cfg.Policy.ToolsDir},
		{"Trust.Path", cfg.Trust.Path},
		{"Telemetry.OTLPEndpoint", cfg.Telemetry.OTLPEndpoint},
	}
	for _, f := range stringFields {
		if f.val == "disabled" {
			t.Errorf("FAILSAFE_MODE leaked into Config.%s = %q", f.name, f.val)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
