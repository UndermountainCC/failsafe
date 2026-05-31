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

func TestAudit_ShowsAllLayers(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(home, "Code", "x")
	os.MkdirAll(repo, 0o755)
	os.MkdirAll(filepath.Join(home, ".config", "failsafe"), 0o755)
	os.WriteFile(filepath.Join(home, ".config", "failsafe", "policy.rego"), []byte(`package guard.user
import future.keywords.if
import future.keywords.contains

block contains {"reason": "user x"} if { true }
`), 0o644)
	os.WriteFile(filepath.Join(repo, ".failsafe.rego"), []byte(`package guard.repo
import future.keywords.if
import future.keywords.contains

block contains {"reason": "repo block"} if { true }
allow_override contains {"reason": "repo override"} if { true }
`), 0o644)

	var out bytes.Buffer
	code := Audit(repo, &out, AuditOptions{Home: home})
	if code != 0 {
		t.Fatalf("exit=%d, out=%q", code, out.String())
	}
	for _, want := range []string{"bundled", "user", "repo", "block rules", "allow_override", "repo override"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in audit output:\n%s", want, out.String())
		}
	}
}
