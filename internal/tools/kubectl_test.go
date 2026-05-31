// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import "testing"

func TestKubectl_Match(t *testing.T) {
	k := NewKubectl()
	for _, ok := range []string{"kubectl", "/usr/local/bin/kubectl"} {
		if !k.Match(ok) {
			t.Errorf("Match(%q) should be true", ok)
		}
	}
	for _, no := range []string{"helm", "kubectx", "kubeconfig"} {
		if k.Match(no) {
			t.Errorf("Match(%q) should be false", no)
		}
	}
}

func TestKubectl_Parse(t *testing.T) {
	k := NewKubectl()
	cases := []struct {
		name string
		args []string
		verb string
		flag string // single flag to spot-check (key=val format)
	}{
		{"plain get", []string{"get", "pods"}, "get", ""},
		{"flag-before-verb context space",
			[]string{"--context", "arn:aws:eks:us-west-2:123456789012:cluster/dev", "get", "pods"},
			"get", "context=arn:aws:eks:us-west-2:123456789012:cluster/dev"},
		{"flag-before-verb context equals",
			[]string{"--context=arn:aws:eks:us-west-2:123456789012:cluster/dev", "get", "pods"},
			"get", "context=arn:aws:eks:us-west-2:123456789012:cluster/dev"},
		{"-n short", []string{"-n", "kube-system", "get", "pods"}, "get", "namespace=kube-system"},
		{"--namespace long", []string{"--namespace", "kube-system", "get", "pods"}, "get", "namespace=kube-system"},
		{"-v=8", []string{"-v=8", "describe", "pod", "foo"}, "describe", "v=8"},
		{"exec -it combined", []string{"exec", "-it", "pod", "--", "bash"}, "exec", ""},
		{"apply with --dry-run=server", []string{"--context", "arn:...:cluster/prod", "apply", "-f", "x.yaml", "--dry-run=server"}, "apply", "dry-run=server"},
		{"apply with --dry-run server (space form)", []string{"--context", "arn:...:cluster/prod", "apply", "-f", "x.yaml", "--dry-run", "server"}, "apply", "dry-run=server"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := k.Parse(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if p.Verb != tc.verb {
				t.Errorf("Verb = %q, want %q (parsed: %+v)", p.Verb, tc.verb, p)
			}
			if tc.flag != "" {
				key, val, _ := splitOnce(tc.flag, "=")
				got, ok := p.Flags[key]
				if !ok {
					t.Errorf("expected flag %q in %v", key, p.Flags)
					return
				}
				if s, _ := got.(string); s != val {
					t.Errorf("flag %q = %v, want %q", key, got, val)
				}
			}
		})
	}
}
