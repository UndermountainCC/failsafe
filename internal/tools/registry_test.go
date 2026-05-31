// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"path/filepath"
	"testing"

	"github.com/UndermountainCC/failsafe/internal/shellparser"
)

type fakeTool struct {
	name string
	want string
}

func (f fakeTool) Name() string                        { return f.name }
func (f fakeTool) Match(t string) bool                 { return filepath.Base(t) == f.want }
func (f fakeTool) Parse(args []string) (Parsed, error) { return Parsed{Verb: "ok"}, nil }
func (f fakeTool) Enrichers() []string                 { return nil }

func TestRegistry_Find(t *testing.T) {
	reg := NewRegistry()
	reg.Add(fakeTool{name: "k", want: "kubectl"})
	reg.Add(fakeTool{name: "g", want: "git"})

	cases := []struct {
		name     string
		call     shellparser.EffectiveCall
		wantTool string
	}{
		{"plain kubectl", shellparser.EffectiveCall{Name: "kubectl", Args: []string{"get", "pods"}}, "k"},
		{"absolute path kubectl", shellparser.EffectiveCall{Name: "/usr/local/bin/kubectl", Args: []string{"get"}}, "k"},
		{"git", shellparser.EffectiveCall{Name: "git", Args: []string{"status"}}, "g"},
		{"echo no match", shellparser.EffectiveCall{Name: "echo", Args: []string{"hi"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool, ok := reg.Find(tc.call)
			if tc.wantTool == "" {
				if ok {
					t.Errorf("expected no match, got tool=%s", tool.Name())
				}
				return
			}
			if !ok {
				t.Fatalf("expected tool %q, got no match", tc.wantTool)
			}
			if tool.Name() != tc.wantTool {
				t.Errorf("tool name = %q, want %q", tool.Name(), tc.wantTool)
			}
		})
	}
}

func TestRegistry_AllPreservesOrder(t *testing.T) {
	reg := NewRegistry()
	reg.Add(fakeTool{name: "k", want: "kubectl"})
	reg.Add(fakeTool{name: "g", want: "git"})
	all := reg.All()
	if len(all) != 2 || all[0].Name() != "k" || all[1].Name() != "g" {
		t.Errorf("All() = %v, want [k g]", all)
	}
}

func TestRegistry_LastAddedWinsOnMatchCollision(t *testing.T) {
	// Spec §3.3: "Later wins on Match() collision." Built-in Go tools are
	// registered first, then bundled YAMLs, then user YAMLs — so user
	// YAMLs override anything earlier with the same Match.
	reg := NewRegistry()
	reg.Add(fakeTool{name: "builtin-kubectl", want: "kubectl"})
	reg.Add(fakeTool{name: "user-kubectl", want: "kubectl"}) // shadows
	tool, ok := reg.Find(shellparser.EffectiveCall{Name: "kubectl"})
	if !ok {
		t.Fatal("expected match")
	}
	if tool.Name() != "user-kubectl" {
		t.Errorf("Find returned %q, want last-added user-kubectl", tool.Name())
	}
}
