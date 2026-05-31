// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package enrich

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// GitEnricher populates fact.git.{remote_url, branch} from a repo's .git/config
// (no shell-out, file IO only). Walks up from CWD to find the .git directory.
type GitEnricher struct{ CWD string }

func (GitEnricher) Name() string { return "git" }

func (g GitEnricher) Enrich(ctx context.Context, fact map[string]any) {
	if ctx.Err() != nil {
		return
	}
	gitDir := findGitDir(g.CWD)
	if gitDir == "" {
		return
	}
	if ctx.Err() != nil {
		return
	}
	cfg, err := os.ReadFile(filepath.Join(gitDir, "config"))
	if err != nil {
		return
	}
	if ctx.Err() != nil {
		return
	}
	headBody, _ := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	branch := parseBranch(string(headBody))
	remoteURL := parseRemoteURL(string(cfg), "origin")
	out := map[string]any{}
	if remoteURL != "" {
		out["remote_url"] = remoteURL
	}
	if branch != "" {
		out["branch"] = branch
	}
	if len(out) > 0 {
		fact["git"] = out
	}
}

func findGitDir(start string) string {
	dir := start
	for {
		if dir == "" {
			return ""
		}
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return gitPath
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func parseBranch(headBody string) string {
	headBody = strings.TrimSpace(headBody)
	const prefix = "ref: refs/heads/"
	if strings.HasPrefix(headBody, prefix) {
		return headBody[len(prefix):]
	}
	return "" // detached HEAD: no branch name
}

func parseRemoteURL(config, remote string) string {
	// Minimal INI parser: find [remote "<name>"] section, read its `url = ...`.
	target := `[remote "` + remote + `"]`
	lines := strings.Split(config, "\n")
	in := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			in = (t == target)
			continue
		}
		if !in {
			continue
		}
		if k, v, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == "url" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
