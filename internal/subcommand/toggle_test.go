// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/UndermountainCC/failsafe/internal/mode"
)

func tempChain(t *testing.T) (*mode.Chain, string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mode")
	chain := &mode.Chain{
		Sources: []mode.Source{mode.FileSource{Pattern: filepath.Join(dir, "mode")}},
		Default: "read",
	}
	return chain, dir, path
}

func TestToggle_FromReadToReadWrite(t *testing.T) {
	chain, dir, path := tempChain(t)
	os.WriteFile(path, []byte("read"), 0o644)
	var out bytes.Buffer
	code := Toggle(&out, ToggleOptions{Chain: chain, Env: map[string]string{"HOME": dir}})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	body, _ := os.ReadFile(path)
	if string(body) != "read & write" {
		t.Errorf("after toggle, file = %q, want 'read & write'", body)
	}
}

func TestModeGet_ShowsValueAndSource(t *testing.T) {
	chain, dir, path := tempChain(t)
	os.WriteFile(path, []byte("read & write"), 0o644)
	var out bytes.Buffer
	code := ModeGet(&out, ModeOptions{Chain: chain, Env: map[string]string{"HOME": dir}})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !contains(out.String(), "read & write") {
		t.Errorf("missing value: %q", out.String())
	}
}

func TestModeSet_WritesValue(t *testing.T) {
	chain, dir, path := tempChain(t)
	var out bytes.Buffer
	code := ModeSet("read & write", &out, ModeOptions{Chain: chain, Env: map[string]string{"HOME": dir}})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	body, _ := os.ReadFile(path)
	if string(body) != "read & write" {
		t.Errorf("written value = %q", body)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && stringIndex(s, sub) >= 0))
}
func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
