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

func TestTrust_AddDotResolvesAncestorRepo(t *testing.T) {
	// Create home with a repo containing .failsafe.rego, then a subdirectory
	// beneath the repo. `trust .` from the subdirectory should find the repo
	// and add it.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(home, "Code", "myrepo")
	sub := filepath.Join(repo, "src", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".failsafe.rego"), []byte(`package guard.repo`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := Trust([]string{"."}, &out, TrustOptions{Home: home, CWD: sub})
	if code != 0 {
		t.Fatalf("exit=%d, out=%q", code, out.String())
	}
	if !strings.Contains(out.String(), repo) {
		t.Errorf("expected output to mention %s, got %q", repo, out.String())
	}
}

func TestTrust_AddDotNoFailsafeErrors(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	sub := filepath.Join(home, "Code", "noconfig")
	os.MkdirAll(sub, 0o755)
	var out bytes.Buffer
	code := Trust([]string{"."}, &out, TrustOptions{Home: home, CWD: sub})
	if code == 0 {
		t.Errorf("expected non-zero exit; out=%q", out.String())
	}
	if !strings.Contains(out.String(), ".failsafe.rego") {
		t.Errorf("error should mention .failsafe.rego: %q", out.String())
	}
}

func TestTrust_ListAfterAdd(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo1 := filepath.Join(home, "Code", "a")
	repo2 := filepath.Join(home, "Code", "b")
	os.MkdirAll(repo1, 0o755)
	os.MkdirAll(repo2, 0o755)
	var out bytes.Buffer
	Trust([]string{repo1}, &out, TrustOptions{Home: home, CWD: repo1})
	out.Reset()
	Trust([]string{repo2}, &out, TrustOptions{Home: home, CWD: repo2})
	out.Reset()
	code := Trust([]string{"list"}, &out, TrustOptions{Home: home})
	if code != 0 {
		t.Fatalf("list exit=%d", code)
	}
	if !strings.Contains(out.String(), repo1) || !strings.Contains(out.String(), repo2) {
		t.Errorf("list should mention both repos, got: %s", out.String())
	}
}

func TestTrust_RemoveThenCheckExits1(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(home, "Code", "x")
	os.MkdirAll(repo, 0o755)
	var out bytes.Buffer
	Trust([]string{repo}, &out, TrustOptions{Home: home, CWD: repo})

	out.Reset()
	if code := Trust([]string{"check", repo}, &out, TrustOptions{Home: home, CWD: repo}); code != 0 {
		t.Errorf("check after add should exit 0, got %d", code)
	}

	out.Reset()
	Trust([]string{"remove", repo}, &out, TrustOptions{Home: home, CWD: repo})

	out.Reset()
	if code := Trust([]string{"check", repo}, &out, TrustOptions{Home: home, CWD: repo}); code == 0 {
		t.Errorf("check after remove should exit non-zero, got %d", code)
	}
}

func TestTrust_CheckCWDDefault(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(home, "Code", "auto")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".failsafe.rego"), []byte(`package guard.repo`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	Trust([]string{repo}, &out, TrustOptions{Home: home, CWD: repo})
	out.Reset()
	// From inside the repo (cwd=repo), `trust check` with no path argument
	// should default to the cwd's repo, which is the trusted one.
	if code := Trust([]string{"check"}, &out, TrustOptions{Home: home, CWD: repo}); code != 0 {
		t.Errorf("check from trusted repo cwd should exit 0, got %d, out=%q", code, out.String())
	}
}
