// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
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
