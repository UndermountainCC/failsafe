// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePolicy(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidate_ValidRepoPolicy(t *testing.T) {
	path := writePolicy(t, ".failsafe.rego", `package guard.repo
import future.keywords.if
import future.keywords.contains

block contains {"reason": "x"} if { true }
allow_override contains {"reason": "y"} if { true }
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{})
	if code != 0 {
		t.Errorf("exit=%d, out=%q", code, out.String())
	}
}

func TestValidate_UserCannotEmitOverride(t *testing.T) {
	path := writePolicy(t, "policy.rego", `package guard.user
import future.keywords.if
import future.keywords.contains

allow_override contains {"reason": "no"} if { true }
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{})
	if code == 0 {
		t.Errorf("expected non-zero exit; out=%q", out.String())
	}
	if !strings.Contains(out.String(), "allow_override") {
		t.Errorf("output should mention allow_override: %q", out.String())
	}
}

func TestValidate_BadPackage(t *testing.T) {
	path := writePolicy(t, ".failsafe.rego", `package wrong.name

import future.keywords.if
import future.keywords.contains

block contains {"reason": "x"} if { true }
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{})
	if code == 0 {
		t.Errorf("expected non-zero on wrong package; out=%q", out.String())
	}
}

func TestValidate_ParseError(t *testing.T) {
	path := writePolicy(t, ".failsafe.rego", `not valid rego at all`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{})
	if code == 0 {
		t.Error("expected non-zero on parse error")
	}
}

func TestValidate_BundledHelperRulesPass(t *testing.T) {
	// Bundled policies use helpers like read_verbs, allowed_dry_run, is_read_action.
	// These must NOT be flagged — only allow_override is reserved outside the repo layer.
	path := writePolicy(t, "kubectl.rego", `package guard.bundled.kubectl

import future.keywords.if
import future.keywords.in
import future.keywords.contains

read_verbs := {"get", "describe"}

block contains {"reason": "blocked"} if {
    input.tool == "kubectl"
    not input.verb in read_verbs
}

allowed_dry_run if {
    input.flags["dry-run"] in {"client", "server"}
}
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{})
	if code != 0 {
		t.Errorf("expected pass with helpers, got exit=%d, out=%q", code, out.String())
	}
}

func TestValidate_RepoCanDeclareBothBlockAndOverride(t *testing.T) {
	path := writePolicy(t, ".failsafe.rego", `package guard.repo

import future.keywords.if
import future.keywords.contains

block contains {"reason": "tighten"} if { true }
allow_override contains {"reason": "loosen"} if { true }
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{})
	if code != 0 {
		t.Errorf("repo should allow both rule types; exit=%d, out=%q", code, out.String())
	}
}

func TestValidate_WarnsOnUnknownFactField(t *testing.T) {
	// `input.repo_name` is not in the fact schema; should warn (not fail).
	path := writePolicy(t, ".failsafe.rego", `package guard.repo

import future.keywords.if
import future.keywords.contains

block contains {"reason": "x"} if { input.repo_name == "infra" }
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{})
	if code != 0 {
		t.Errorf("warnings shouldn't fail without --strict; exit=%d", code)
	}
	if !strings.Contains(out.String(), "input.repo_name") {
		t.Errorf("expected warning about input.repo_name; out=%q", out.String())
	}
	if !strings.Contains(out.String(), "with") || !strings.Contains(out.String(), "warning") {
		t.Errorf("expected output to summarize warnings; out=%q", out.String())
	}
}

func TestValidate_StrictPromotesWarningsToErrors(t *testing.T) {
	path := writePolicy(t, ".failsafe.rego", `package guard.repo

import future.keywords.if
import future.keywords.contains

block contains {"reason": "x"} if { input.repo_name == "x" }
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{Strict: true})
	if code == 0 {
		t.Errorf("--strict should fail when warnings are present; out=%q", out.String())
	}
}

func TestValidate_KnownFactFieldsPass(t *testing.T) {
	// References to known fields should NOT warn.
	path := writePolicy(t, ".failsafe.rego", `package guard.repo

import future.keywords.if
import future.keywords.contains

block contains {"reason": "x"} if {
    input.tool == "kubectl"
    input.verb == "apply"
    input.flags["dry-run"] == "server"
    input.git.remote_url == "x"
}
`)
	var out bytes.Buffer
	code := Validate(path, &out, ValidateOptions{Strict: true})
	if code != 0 {
		t.Errorf("known fact fields should not warn under --strict; exit=%d, out=%q", code, out.String())
	}
}
