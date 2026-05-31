// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"strings"
	"testing"
)

func TestExplain_BlockShowsReason(t *testing.T) {
	var out bytes.Buffer
	code := Explain([]string{"kubectl", "apply", "-f", "x.yaml"}, &out, ExplainOptions{Home: t.TempDir(), CWD: t.TempDir(), Mode: "read"})
	if code != 0 {
		t.Fatalf("exit=%d, out=%q", code, out.String())
	}
	if !strings.Contains(out.String(), "Decision: BLOCK") {
		t.Errorf("missing BLOCK in output: %s", out.String())
	}
	if !strings.Contains(out.String(), "kubectl apply blocked in read mode") {
		t.Errorf("missing reason: %s", out.String())
	}
}

// joinShellArgs must quote shell metacharacters so a `;` passed as a literal
// argv element doesn't get re-parsed as a statement separator after we
// reassemble argv → command string → shellparser.
func TestExplain_QuotesSemicolonInArgs(t *testing.T) {
	var out bytes.Buffer
	Explain([]string{"echo", ";", "rm", "-rf", "/"}, &out, ExplainOptions{
		Home: t.TempDir(), CWD: t.TempDir(), Mode: "read",
	})
	// With proper quoting, this is ONE command (echo with arg ;), not two.
	// We expect "echo" as the only call. "rm" should NOT appear as a separate
	// "call 2" because the ; was quoted.
	if strings.Contains(out.String(), "rm") && strings.Contains(out.String(), "call 2") {
		t.Errorf("rm appeared as a separate call; ; was not quoted properly:\n%s", out.String())
	}
}
