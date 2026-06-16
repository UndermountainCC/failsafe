// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package trust

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrAlreadyTrusted = errors.New("repo already trusted")
	ErrNotTrusted     = errors.New("repo not in trust list")
)

type Trust struct {
	file  string
	repos []TrustedRepo
}

type TrustedRepo struct {
	Path    string `yaml:"path"`
	AddedAt string `yaml:"added_at"`
	Reason  string `yaml:"reason,omitempty"`
}

type fileShape struct {
	Repos []TrustedRepo `yaml:"repos"`
}

// Load reads the trust file from $HOME/.config/failsafe/trusted-repos.yaml.
// A missing file is not an error; you get an empty Trust ready for Add.
func Load(home string) (*Trust, error) {
	path := filepath.Join(home, ".config", "failsafe", "trusted-repos.yaml")
	t := &Trust{file: path}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return t, nil
		}
		return nil, err
	}
	var f fileShape
	if err := yaml.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	t.repos = f.Repos
	return t, nil
}

func (t *Trust) IsTrusted(repoPath string) bool {
	canonical := resolveRepoIdentity(repoPath)
	for _, r := range t.repos {
		if filepath.Clean(r.Path) == canonical {
			return true
		}
	}
	return false
}

func (t *Trust) Add(repoPath, reason string) error {
	canonical := resolveRepoIdentity(repoPath)
	if t.IsTrusted(canonical) {
		return ErrAlreadyTrusted
	}
	t.repos = append(t.repos, TrustedRepo{
		Path:    canonical,
		AddedAt: time.Now().UTC().Format(time.RFC3339),
		Reason:  reason,
	})
	return t.save()
}

func (t *Trust) Remove(repoPath string) error {
	canonical := resolveRepoIdentity(repoPath)
	out := t.repos[:0]
	found := false
	for _, r := range t.repos {
		if filepath.Clean(r.Path) == canonical {
			found = true
			continue
		}
		out = append(out, r)
	}
	if !found {
		return ErrNotTrusted
	}
	t.repos = out
	return t.save()
}

func (t *Trust) List() []TrustedRepo {
	return append([]TrustedRepo(nil), t.repos...)
}

func (t *Trust) save() error {
	if err := os.MkdirAll(filepath.Dir(t.file), 0o755); err != nil {
		return err
	}
	body, err := yaml.Marshal(fileShape{Repos: t.repos})
	if err != nil {
		return err
	}
	tmp := t.file + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, t.file)
}

// resolveRepoIdentity maps a path to the canonical identity of its git repo so
// every worktree of a repo shares one trust key. A LINKED worktree's `.git` is
// a FILE ("gitdir: <gitdir>"); the MAIN worktree's `.git` is a directory. For a
// linked worktree we resolve back to the main repo's working tree via the
// worktree gitdir's `commondir` file — no `git` exec. Main worktrees and
// non-git paths are returned unchanged (Abs+Clean).
//
// EvalSymlinks is used so that symlinked prefixes (e.g. macOS /tmp → /private/tmp)
// are normalised consistently across the main repo path and the absolute gitdir
// path written into linked worktrees' .git files.
func resolveRepoIdentity(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	abs = filepath.Clean(abs)
	// Resolve symlinks so the stored key matches the real path that git writes
	// into worktree .git files (e.g. /tmp → /private/tmp on macOS).
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	gitPath := filepath.Join(abs, ".git")
	info, err := os.Stat(gitPath)
	if err != nil || info.IsDir() {
		return abs // main worktree / non-git: unchanged
	}
	body, err := os.ReadFile(gitPath) // ".git" is a file → linked worktree
	if err != nil {
		return abs
	}
	line := strings.TrimSpace(string(body))
	gitdir, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return abs
	}
	wtGitDir := strings.TrimSpace(gitdir)
	if !filepath.IsAbs(wtGitDir) {
		wtGitDir = filepath.Clean(filepath.Join(abs, wtGitDir))
	}
	// wtGitDir = <commonGit>/worktrees/<name>; `commondir` points to <commonGit>.
	commonGit := filepath.Clean(filepath.Dir(filepath.Dir(wtGitDir))) // fallback: strip /worktrees/<name>
	if cb, err := os.ReadFile(filepath.Join(wtGitDir, "commondir")); err == nil {
		cd := strings.TrimSpace(string(cb))
		if !filepath.IsAbs(cd) {
			cd = filepath.Join(wtGitDir, cd)
		}
		commonGit = filepath.Clean(cd)
	}
	// main working tree = parent of the common .git dir (non-bare layout).
	return filepath.Clean(filepath.Dir(commonGit))
}
