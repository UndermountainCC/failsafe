// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package enrich

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGit_PopulatesRemoteAndBranch(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	gitdir := filepath.Join(repo, ".git")
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "config"), []byte(`[remote "origin"]
	url = git@github.com:acme/infra.git
[branch "master"]
	remote = origin
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/master\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := map[string]any{}
	GitEnricher{CWD: repo}.Enrich(context.Background(), out)
	g, _ := out["git"].(map[string]any)
	if g["remote_url"] != "git@github.com:acme/infra.git" {
		t.Errorf("remote_url = %v", g["remote_url"])
	}
	if g["branch"] != "master" {
		t.Errorf("branch = %v", g["branch"])
	}
}

func TestGit_NotARepoIsNoop(t *testing.T) {
	dir := t.TempDir()
	out := map[string]any{}
	GitEnricher{CWD: dir}.Enrich(context.Background(), out)
	if _, ok := out["git"]; ok {
		t.Errorf("expected no git key when cwd is not a repo")
	}
}
