// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"strings"
	"testing"
)

const gitYAML = `
name: git
match: ["git"]
global_flags:
  - { long: "C",          short: "C", takes_value: true, style: short }
  - { long: "git-dir",    takes_value: true }
  - { long: "no-pager",   takes_value: false }
verbs:
  push:    {}
  status:  {}
  remote:
    subverbs: [add, remove, get-url, list]
enrich: [git]
`

func loadGit(t *testing.T) Tool {
	t.Helper()
	tool, err := LoadYAMLTool(strings.NewReader(gitYAML))
	if err != nil {
		t.Fatalf("LoadYAMLTool: %v", err)
	}
	return tool
}

func TestYAMLTool_Match(t *testing.T) {
	tool := loadGit(t)
	if !tool.Match("git") {
		t.Error("Match(git) should be true")
	}
	if !tool.Match("/usr/local/bin/git") {
		t.Error("Match(/usr/local/bin/git) should be true")
	}
	if tool.Match("kubectl") {
		t.Error("Match(kubectl) should be false")
	}
}

func TestYAMLTool_Parse(t *testing.T) {
	tool := loadGit(t)
	cases := []struct {
		name string
		args []string
		want Parsed
	}{
		{"plain status", []string{"status"}, Parsed{Verb: "status"}},
		{"flag before verb", []string{"-C", "/tmp/repo", "status"},
			Parsed{Verb: "status", Flags: map[string]interface{}{"C": "/tmp/repo"}}},
		{"long flag with =", []string{"--git-dir=/x/y", "push"},
			Parsed{Verb: "push", Flags: map[string]interface{}{"git-dir": "/x/y"}}},
		{"long flag space form", []string{"--git-dir", "/x/y", "push"},
			Parsed{Verb: "push", Flags: map[string]interface{}{"git-dir": "/x/y"}}},
		{"boolean flag", []string{"--no-pager", "status"},
			Parsed{Verb: "status", Flags: map[string]interface{}{"no-pager": true}}},
		{"subverb", []string{"remote", "get-url", "origin"},
			Parsed{Verb: "remote", Subverb: "get-url", Positional: []string{"origin"}}},
		// Unknown long flags are stored as bool=true and DO NOT consume the
		// next token. Unknown arity is unknowable from the schema, and
		// consuming would misparse common boolean flags (e.g.
		// `terraform --no-color destroy`). The next token falls through to
		// the normal verb/positional path.
		{"unknown long flag without value-consume",
			[]string{"--unknown", "value", "status"},
			Parsed{Verb: "value", Flags: map[string]interface{}{"unknown": true}}},
		{"unknown long flag with another flag", []string{"--unknown", "--git-dir=/x", "status"},
			Parsed{Verb: "status", Flags: map[string]interface{}{"unknown": true, "git-dir": "/x"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tool.Parse(tc.args)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.Verb != tc.want.Verb {
				t.Errorf("Verb = %q, want %q", got.Verb, tc.want.Verb)
			}
			if got.Subverb != tc.want.Subverb {
				t.Errorf("Subverb = %q, want %q", got.Subverb, tc.want.Subverb)
			}
			for k, v := range tc.want.Flags {
				if got.Flags[k] != v {
					t.Errorf("Flags[%q] = %v, want %v", k, got.Flags[k], v)
				}
			}
		})
	}
}

func TestYAMLTool_TerraformGNUShort(t *testing.T) {
	const tfYAML = `
name: terraform
match: ["terraform", "tofu"]
global_flags:
  - { long: "chdir", takes_value: true, style: gnu_short }
verbs:
  plan:  {}
  apply: {}
  state:
    subverbs: [list, show]
`
	tool, err := LoadYAMLTool(strings.NewReader(tfYAML))
	if err != nil {
		t.Fatalf("LoadYAMLTool: %v", err)
	}
	got, _ := tool.Parse([]string{"-chdir=modules/foo", "plan"})
	if got.Verb != "plan" {
		t.Errorf("verb = %q, want plan; got=%+v", got.Verb, got)
	}
	if got.Flags["chdir"] != "modules/foo" {
		t.Errorf("chdir flag = %v, want modules/foo", got.Flags["chdir"])
	}
}

// An unknown long flag MUST NOT consume the next argv as its value, even
// when that token doesn't start with `-`. Otherwise common boolean flags
// like `terraform --no-color destroy` parse as Verb="" with the destroy
// hidden in Flags["no-color"], and bundled rego's `input.verb != ""` gate
// would let `destroy` slip through unblocked. Locks in fail-closed behavior.
func TestYAMLTool_UnknownBooleanFlagDoesNotConsumeVerb(t *testing.T) {
	const tfYAML = `
name: terraform
match: ["terraform", "tofu"]
global_flags:
  - { long: "chdir", takes_value: true, style: gnu_short }
verbs:
  destroy: {}
  plan: {}
`
	tool, err := LoadYAMLTool(strings.NewReader(tfYAML))
	if err != nil {
		t.Fatalf("LoadYAMLTool: %v", err)
	}
	got, _ := tool.Parse([]string{"--no-color", "destroy"})
	if got.Verb != "destroy" {
		t.Errorf("Verb = %q, want 'destroy' (got=%+v)", got.Verb, got)
	}
	if got.Flags["no-color"] != true {
		t.Errorf("--no-color should be bool=true, got %v", got.Flags["no-color"])
	}
}

// A gnu_short flag declared as takes_value=false MUST NOT swallow the next
// token. Previously the space-form branch consumed the next argv
// unconditionally, which would steal the verb.
func TestYAMLTool_GNUShortBooleanDoesNotConsumeNext(t *testing.T) {
	const tfYAML = `
name: terraform
match: ["terraform"]
global_flags:
  - { long: "verbose", takes_value: false, style: gnu_short }
verbs:
  plan: {}
`
	tool, err := LoadYAMLTool(strings.NewReader(tfYAML))
	if err != nil {
		t.Fatalf("LoadYAMLTool: %v", err)
	}
	got, _ := tool.Parse([]string{"-verbose", "plan"})
	if got.Verb != "plan" {
		t.Errorf("verb = %q, want plan; got=%+v", got.Verb, got)
	}
	if got.Flags["verbose"] != true {
		t.Errorf("verbose flag = %v, want true (boolean); got=%+v", got.Flags["verbose"], got)
	}
}
