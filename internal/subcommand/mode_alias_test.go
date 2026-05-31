// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"os"
	"testing"
)

func TestNormalizeMode(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		valid bool
	}{
		{"rw", "read & write", true},
		{"ro", "read", true},
		{"r", "read", true},
		{"w", "read & write", true},
		{"RW", "read & write", true},  // case-insensitive
		{"  ro  ", "read", true},      // trimmed
		{"read", "read", true},        // canonical still accepted
		{"read & write", "read & write", true},
		{"read+write", "read & write", true},
		{"readwrite", "read & write", true},
		{"nonsense", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeMode(c.in)
		if ok != c.valid || got != c.want {
			t.Errorf("normalizeMode(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.valid)
		}
	}
}

// The whole point of the change: `mode set rw` must write the canonical value
// the Rego policies match on, not the alias.
func TestModeSet_ShortAliasesWriteCanonical(t *testing.T) {
	cases := map[string]string{"rw": "read & write", "ro": "read"}
	for alias, canon := range cases {
		chain, dir, path := tempChain(t)
		var out bytes.Buffer
		if code := ModeSet(alias, &out, ModeOptions{Chain: chain, Env: map[string]string{"HOME": dir}}); code != 0 {
			t.Fatalf("ModeSet(%q) exit=%d (%s)", alias, code, out.String())
		}
		body, _ := os.ReadFile(path)
		if string(body) != canon {
			t.Errorf("ModeSet(%q) wrote %q, want canonical %q", alias, body, canon)
		}
	}
}

func TestModeSet_RejectsUnknown(t *testing.T) {
	chain, dir, _ := tempChain(t)
	var out bytes.Buffer
	if code := ModeSet("write-everything", &out, ModeOptions{Chain: chain, Env: map[string]string{"HOME": dir}}); code != 2 {
		t.Errorf("ModeSet(unknown) exit=%d, want 2; out=%q", code, out.String())
	}
}
