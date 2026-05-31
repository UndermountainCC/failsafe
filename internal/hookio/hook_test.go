// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package hookio

import (
	"bytes"
	"strings"
	"testing"
)

func TestRead_TypicalInput(t *testing.T) {
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"kubectl get pods"},"cwd":"/tmp/x","session_id":"s1"}`)
	got, err := Read(in)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.ToolInput.Command != "kubectl get pods" {
		t.Errorf("Command = %q", got.ToolInput.Command)
	}
	if got.CWD != "/tmp/x" {
		t.Errorf("CWD = %q", got.CWD)
	}
	if got.SessionID != "s1" {
		t.Errorf("SessionID = %q", got.SessionID)
	}
}

func TestWriteBlock_FormatsForClaude(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteBlock(&buf, "kubectl apply blocked: prod cluster"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"decision"`) || !strings.Contains(out, `"block"`) {
		t.Errorf("missing decision/block: %q", out)
	}
	if !strings.Contains(out, "prod cluster") {
		t.Errorf("missing reason: %q", out)
	}
}

func TestWriteBlock_WithOverrideContext(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAllowWithOverride(&buf, "sandbox repo: force-push permitted"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "additionalContext") {
		t.Errorf("missing additionalContext: %q", out)
	}
	if !strings.Contains(out, "sandbox repo") {
		t.Errorf("missing override reason: %q", out)
	}
}
