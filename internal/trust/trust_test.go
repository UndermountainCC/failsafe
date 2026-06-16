// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package trust

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	tr, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.List()) != 0 {
		t.Errorf("expected empty list")
	}
}

func TestAddCheckRemove(t *testing.T) {
	dir := t.TempDir()
	tr, _ := Load(dir)
	repo := filepath.Join(dir, "repo")
	if tr.IsTrusted(repo) {
		t.Error("should not be trusted before add")
	}
	if err := tr.Add(repo, "test"); err != nil {
		t.Fatal(err)
	}
	if !tr.IsTrusted(repo) {
		t.Error("should be trusted after add")
	}
	// Reload from disk; should persist.
	tr2, _ := Load(dir)
	if !tr2.IsTrusted(repo) {
		t.Error("should be trusted after reload")
	}
	if err := tr2.Remove(repo); err != nil {
		t.Fatal(err)
	}
	tr3, _ := Load(dir)
	if tr3.IsTrusted(repo) {
		t.Error("should not be trusted after remove + reload")
	}
}

func TestPathCanonicalization(t *testing.T) {
	dir := t.TempDir()
	tr, _ := Load(dir)
	repo := filepath.Join(dir, "repo")
	tr.Add(repo, "")
	// Querying with a non-canonical path should still match.
	if !tr.IsTrusted(filepath.Join(dir, "..", filepath.Base(dir), "repo")) {
		t.Error("non-canonical path should resolve to trusted")
	}
}

// TestWorktreeTrust verifies that trusting the main repo also covers linked
// worktrees, and that attempting to trust from the worktree path returns
// ErrAlreadyTrusted when the main repo is already trusted.
func TestWorktreeTrust(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH; skipping worktree trust test")
	}

	tmp := t.TempDir()
	mainRepo := filepath.Join(tmp, "main")
	wtPath := filepath.Join(tmp, "wt")

	// git init main repo.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitBin, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", mainRepo)
	run("-C", mainRepo, "config", "user.email", "test@test.com")
	run("-C", mainRepo, "config", "user.name", "Test")

	// Need at least one commit so `git worktree add` works.
	regoFile := filepath.Join(mainRepo, ".failsafe.rego")
	if err := os.WriteFile(regoFile, []byte("package failsafe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("-C", mainRepo, "add", ".failsafe.rego")
	run("-C", mainRepo, "commit", "-m", "init")

	// Add a linked worktree.
	run("-C", mainRepo, "worktree", "add", wtPath)

	// Use a fresh trust store.
	trustDir := t.TempDir()
	tr, err := Load(trustDir)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Trust the main repo.
	if err := tr.Add(mainRepo, "test"); err != nil {
		t.Fatalf("Add(mainRepo): %v", err)
	}

	// 2. IsTrusted(worktreePath) must return true.
	if !tr.IsTrusted(wtPath) {
		t.Error("worktree path should be trusted because main repo is trusted")
	}

	// 3. Add(worktreePath) when main is already trusted → ErrAlreadyTrusted.
	if err := tr.Add(wtPath, "from wt"); !errors.Is(err, ErrAlreadyTrusted) {
		t.Errorf("expected ErrAlreadyTrusted, got %v", err)
	}
}

// TestResolveRepoIdentityNonGit verifies that a plain directory (no .git) is
// returned unchanged by resolveRepoIdentity (modulo symlink resolution).
func TestResolveRepoIdentityNonGit(t *testing.T) {
	plain := t.TempDir()
	got := resolveRepoIdentity(plain)
	// EvalSymlinks is applied inside resolveRepoIdentity, so compute want the
	// same way to stay portable across systems where /tmp is a symlink.
	want, _ := filepath.Abs(plain)
	want = filepath.Clean(want)
	if real, err := filepath.EvalSymlinks(want); err == nil {
		want = real
	}
	if got != want {
		t.Errorf("resolveRepoIdentity(%q) = %q; want %q", plain, got, want)
	}

	// Trust behaviour: add plain dir, check it is trusted, a different plain
	// dir is not.
	trustDir := t.TempDir()
	tr, _ := Load(trustDir)
	tr.Add(plain, "")
	if !tr.IsTrusted(plain) {
		t.Error("plain dir should be trusted after add")
	}
	other := t.TempDir()
	if tr.IsTrusted(other) {
		t.Error("unrelated plain dir must not be trusted")
	}
}
