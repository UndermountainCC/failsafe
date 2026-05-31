// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"failsafe", "--version"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "failsafe") {
		t.Errorf("stdout did not mention failsafe: %q", stdout.String())
	}
}

func TestRun_Report(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "decisions.jsonl")
	// A recent record so it falls inside the default 7d window (run uses time.Now()).
	ts := time.Now().UTC().Format(time.RFC3339)
	line := `{"ts":"` + ts + `","decision":"block","tool":"kubectl","verb":"delete","session":{"agent_session_id":"s"}}` + "\n"
	if err := os.WriteFile(logPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAILSAFE_LOG", logPath) // report must read where the hook writes

	var stdout, stderr bytes.Buffer
	code := run([]string{"failsafe", "report", "--format", "json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kubectl"`) {
		t.Errorf("report did not include the logged kubectl decision:\n%s", stdout.String())
	}
}
