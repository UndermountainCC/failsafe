// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolsList_ShowsAll(t *testing.T) {
	var out bytes.Buffer
	code := ToolsList(&out, ToolsListOptions{Home: t.TempDir()})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	for _, want := range []string{"kubectl", "helm", "terraform", "aws", "git"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in output:\n%s", want, out.String())
		}
	}
	if !strings.Contains(out.String(), "built-in Go") {
		t.Error("missing 'built-in Go' tag")
	}
	if !strings.Contains(out.String(), "bundled YAML") {
		t.Error("missing 'bundled YAML' tag")
	}
}

// TestToolsList_CustomToolsDir proves that ToolsListOptions.ToolsDir (sourced
// from cfg.Policy.ToolsDir in main.go) is used instead of the Home-derived
// default. A YAML file placed in the custom directory must appear in the output;
// a file placed in the default Home-derived directory must NOT appear.
func TestToolsList_CustomToolsDir(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	customToolsDir := filepath.Join(dir, "custom-tools")
	if err := os.MkdirAll(customToolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a fake tool YAML into the custom dir.
	if err := os.WriteFile(filepath.Join(customToolsDir, "mytool.yaml"), []byte("name: mytool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a fake tool YAML into the home-derived default dir (should NOT appear).
	defaultToolsDir := filepath.Join(home, ".config", "failsafe", "tools")
	if err := os.MkdirAll(defaultToolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultToolsDir, "defaulttool.yaml"), []byte("name: defaulttool\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	code := ToolsList(&out, ToolsListOptions{Home: home, ToolsDir: customToolsDir})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out.String(), "mytool") {
		t.Errorf("custom ToolsDir tool 'mytool' should appear; got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "defaulttool") {
		t.Errorf("default home-derived tool 'defaulttool' must not appear when ToolsDir is set; got:\n%s", out.String())
	}
}
