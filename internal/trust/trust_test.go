// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package trust

import (
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
