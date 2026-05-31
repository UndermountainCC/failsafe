// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package trust

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	canonical, err := filepath.Abs(repoPath)
	if err != nil {
		return false
	}
	canonical = filepath.Clean(canonical)
	for _, r := range t.repos {
		if filepath.Clean(r.Path) == canonical {
			return true
		}
	}
	return false
}

func (t *Trust) Add(repoPath, reason string) error {
	canonical, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	canonical = filepath.Clean(canonical)
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
	canonical, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	canonical = filepath.Clean(canonical)
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
