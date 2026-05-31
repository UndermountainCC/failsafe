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

func TestRunCorpus_PassingCase(t *testing.T) {
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "case1")
	os.MkdirAll(caseDir, 0o755)
	os.WriteFile(filepath.Join(caseDir, "fact.json"),
		[]byte(`{"mode":"read","tool":"kubectl","verb":"get"}`), 0o644)
	os.WriteFile(filepath.Join(caseDir, "expected.json"),
		[]byte(`{"block":false}`), 0o644)

	var out bytes.Buffer
	code := TestCorpus(dir, &out, TestOptions{Home: t.TempDir()})
	if code != 0 {
		t.Errorf("exit=%d, out=%q", code, out.String())
	}
	if !strings.Contains(out.String(), "PASS") {
		t.Errorf("expected PASS in output: %s", out.String())
	}
}

func TestRunCorpus_FailingCase(t *testing.T) {
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "case2")
	os.MkdirAll(caseDir, 0o755)
	os.WriteFile(filepath.Join(caseDir, "fact.json"),
		[]byte(`{"mode":"read","tool":"kubectl","verb":"apply"}`), 0o644)
	// expected says allow, but bundled blocks → fail
	os.WriteFile(filepath.Join(caseDir, "expected.json"),
		[]byte(`{"block":false}`), 0o644)
	var out bytes.Buffer
	code := TestCorpus(dir, &out, TestOptions{Home: t.TempDir()})
	if code == 0 {
		t.Errorf("expected non-zero exit for failing case")
	}
}
