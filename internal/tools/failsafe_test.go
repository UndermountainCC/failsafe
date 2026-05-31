// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import "testing"

func TestFailsafe_Match(t *testing.T) {
	lg := NewFailsafeTool()
	for _, ok := range []string{"failsafe", "/Users/you/go/bin/failsafe"} {
		if !lg.Match(ok) {
			t.Errorf("Match(%q) should be true", ok)
		}
	}
	for _, no := range []string{"kubectl", "failsafex", "guard"} {
		if lg.Match(no) {
			t.Errorf("Match(%q) should be false", no)
		}
	}
}

func TestFailsafe_Parse(t *testing.T) {
	lg := NewFailsafeTool()
	cases := []struct {
		name string
		args []string
		verb string
		sub  string
	}{
		{"toggle no args", []string{"toggle"}, "toggle", ""},
		{"mode get", []string{"mode", "get"}, "mode", "get"},
		{"mode set value", []string{"mode", "set", "read"}, "mode", "set"},
		{"trust list", []string{"trust", "list"}, "trust", "list"},
		{"trust check", []string{"trust", "check"}, "trust", "check"},
		{"trust remove path", []string{"trust", "remove", "/some/path"}, "trust", "remove"},
		{"tools list", []string{"tools", "list"}, "tools", "list"},
		{"policies list", []string{"policies", "list"}, "policies", "list"},
		{"explain command tail", []string{"explain", "kubectl", "get", "pods"}, "explain", ""},
		{"validate strict", []string{"validate", "--strict", "/x.rego"}, "validate", ""},
		{"hook lifecycle", []string{"hook"}, "hook", ""},
		{"mcp lifecycle", []string{"mcp"}, "mcp", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lg.Parse(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if got.Verb != tc.verb {
				t.Errorf("Verb = %q, want %q (parsed: %+v)", got.Verb, tc.verb, got)
			}
			if got.Subverb != tc.sub {
				t.Errorf("Subverb = %q, want %q (parsed: %+v)", got.Subverb, tc.sub, got)
			}
		})
	}
}
