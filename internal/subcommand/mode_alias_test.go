// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"os"
	"testing"
)

func TestNormalizeModeMatrix(t *testing.T) {
	enabled := []string{"enabled", "enable", "on", "closed", "close", "lock", "ro", "r", "read", "safe"}
	disabled := []string{"disabled", "disable", "off", "open", "unlock", "rw", "w", "write", "sudo"}
	for _, a := range enabled {
		if got, ok := normalizeMode(a); !ok || got != "enabled" {
			t.Fatalf("%q -> %q,%v; want enabled", a, got, ok)
		}
	}
	for _, a := range disabled {
		if got, ok := normalizeMode(a); !ok || got != "disabled" {
			t.Fatalf("%q -> %q,%v; want disabled", a, got, ok)
		}
	}
	if _, ok := normalizeMode("bogus"); ok {
		t.Fatal("bogus should not normalize")
	}
}

// The whole point of the change: `mode set rw` must write the canonical value
// the Rego policies match on, not the alias.
func TestModeSet_ShortAliasesWriteCanonical(t *testing.T) {
	cases := map[string]string{"rw": "disabled", "ro": "enabled"}
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
