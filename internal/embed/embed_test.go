// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package embed

import (
	"strings"
	"testing"
)

func TestBundledTools_Listed(t *testing.T) {
	got := BundledToolNames()
	want := map[string]bool{"terraform.yaml": true, "aws.yaml": true, "git.yaml": true}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected bundled tool: %s", n)
		}
		delete(want, n)
	}
	if len(want) > 0 {
		t.Errorf("missing bundled tools: %v", want)
	}
}

func TestBundledPolicies_Listed(t *testing.T) {
	got := BundledPolicyNames()
	if len(got) != 6 {
		t.Errorf("expected 6 bundled policies, got %d (%v)", len(got), got)
	}
	for _, n := range got {
		if !strings.HasSuffix(n, ".rego") {
			t.Errorf("non-rego in bundled policies: %s", n)
		}
	}
}

func TestReadBundledTool(t *testing.T) {
	body, err := ReadBundledTool("git.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "name: git") {
		t.Errorf("git.yaml content unexpected")
	}
}
