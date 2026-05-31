// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import "testing"

func TestHelm_Match(t *testing.T) {
	h := NewHelm()
	if !h.Match("helm") {
		t.Error("Match(helm) should be true")
	}
	if h.Match("kubectl") {
		t.Error("Match(kubectl) should be false")
	}
}

func TestHelm_Parse(t *testing.T) {
	h := NewHelm()
	cases := []struct {
		name string
		args []string
		verb string
	}{
		{"plain list", []string{"list"}, "list"},
		{"flag-before-verb namespace", []string{"--namespace", "ns", "list"}, "list"},
		{"-n short before verb", []string{"-n", "ns", "status", "myrelease"}, "status"},
		{"install", []string{"--namespace", "ns", "install", "myrel", "chart"}, "install"},
		{"repo subverb", []string{"repo", "list"}, "repo"},
		// Chart-customization value flags must be recognized so they
		// don't swallow positionals or get mistaken for booleans.
		{"install with --set space form", []string{"install", "--set", "key=val", "myrel", "chart"}, "install"},
		{"install with -f values.yaml short form", []string{"install", "-f", "values.yaml", "myrel", "chart"}, "install"},
		{"install with --values long form", []string{"install", "--values", "values.yaml", "myrel", "chart"}, "install"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := h.Parse(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if p.Verb != tc.verb {
				t.Errorf("Verb = %q, want %q (parsed: %+v)", p.Verb, tc.verb, p)
			}
		})
	}
}

// Verify --set actually captures its value (not just that it's not a
// boolean). Policy rules may want to inspect what a chart configures.
func TestHelm_SetValueCaptured(t *testing.T) {
	h := NewHelm()
	p, _ := h.Parse([]string{"install", "--set", "image.tag=v1", "myrel", "chart"})
	if p.Flags["set"] != "image.tag=v1" {
		t.Errorf("expected --set to capture 'image.tag=v1', got %v", p.Flags["set"])
	}
}
