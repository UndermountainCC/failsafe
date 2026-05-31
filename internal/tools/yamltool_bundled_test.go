// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadBundled(t *testing.T, name string) Tool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "internal", "embed", "tools", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	tool, err := LoadYAMLTool(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return tool
}

func TestBundled_Terraform(t *testing.T) {
	tf := loadBundled(t, "terraform.yaml")
	cases := []struct {
		args []string
		verb string
		sub  string
	}{
		{[]string{"plan"}, "plan", ""},
		{[]string{"-chdir=modules/foo", "plan"}, "plan", ""},
		{[]string{"-chdir", "modules/foo", "apply"}, "apply", ""},
		{[]string{"state", "list"}, "state", "list"},
		{[]string{"destroy"}, "destroy", ""},
	}
	for _, tc := range cases {
		got, _ := tf.Parse(tc.args)
		if got.Verb != tc.verb || got.Subverb != tc.sub {
			t.Errorf("Parse(%v) = {Verb: %q, Subverb: %q}, want {%q, %q}", tc.args, got.Verb, got.Subverb, tc.verb, tc.sub)
		}
	}
}

func TestBundled_AWS(t *testing.T) {
	a := loadBundled(t, "aws.yaml")
	cases := []struct {
		args []string
		verb string
		sub  string
	}{
		{[]string{"sts", "get-caller-identity"}, "sts", "get-caller-identity"},
		{[]string{"--region", "us-west-2", "s3", "ls"}, "s3", "ls"},
		{[]string{"eks", "describe-cluster", "--name", "x"}, "eks", "describe-cluster"},
	}
	for _, tc := range cases {
		got, _ := a.Parse(tc.args)
		if got.Verb != tc.verb || got.Subverb != tc.sub {
			t.Errorf("Parse(%v) = {Verb: %q, Subverb: %q}, want {%q, %q}", tc.args, got.Verb, got.Subverb, tc.verb, tc.sub)
		}
	}
}

func TestBundled_Git(t *testing.T) {
	g := loadBundled(t, "git.yaml")
	cases := []struct {
		args []string
		verb string
		sub  string
	}{
		{[]string{"status"}, "status", ""},
		{[]string{"-C", "/tmp/x", "status"}, "status", ""},
		{[]string{"remote", "get-url", "origin"}, "remote", "get-url"},
		{[]string{"push", "--force", "origin", "main"}, "push", ""},
	}
	for _, tc := range cases {
		got, _ := g.Parse(tc.args)
		if got.Verb != tc.verb || got.Subverb != tc.sub {
			t.Errorf("Parse(%v) = {Verb: %q, Subverb: %q}, want {%q, %q}", tc.args, got.Verb, got.Subverb, tc.verb, tc.sub)
		}
	}
}
