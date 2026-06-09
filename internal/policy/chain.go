// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/ast"
)

// DiscoverOpts configures policy chain assembly. IsTrusted gates whether a
// repo's allow_override rules survive into the engine; the chain *always*
// loads block rules from the repo regardless of trust.
type DiscoverOpts struct {
	BundledLoader func() ([]Module, error)
	Home          string
	CWD           string

	// UserPolicyPath overrides the default user policy path
	// ($HOME/.config/failsafe/policy.rego). When non-empty, this path is used
	// instead of deriving it from Home. Path must be fully expanded (no tildes).
	UserPolicyPath string

	// IsTrusted reports whether a repo path (the directory containing the
	// .failsafe.rego) has been explicitly trusted via `failsafe trust`.
	// Nil → all repos are untrusted (safe default).
	IsTrusted func(repoPath string) bool

	// WarnUntrusted is called once per encounter of an untrusted repo whose
	// .failsafe.rego declares allow_override rules. Nil → write to os.Stderr.
	WarnUntrusted func(repoPath, file string)
}

// Discover assembles bundled + user + repo modules from the filesystem.
// Repo modules are tagged with Trusted reflecting opts.IsTrusted.
func Discover(opts DiscoverOpts) ([]Module, error) {
	var out []Module

	if opts.BundledLoader != nil {
		mods, err := opts.BundledLoader()
		if err != nil {
			return nil, err
		}
		out = append(out, mods...)
	}

	if opts.UserPolicyPath != "" || opts.Home != "" {
		userPath := opts.UserPolicyPath
		if userPath == "" {
			userPath = filepath.Join(opts.Home, ".config", "failsafe", "policy.rego")
		}
		body, err := os.ReadFile(userPath)
		switch {
		case err == nil:
			out = append(out, Module{Layer: LayerUser, File: userPath, Body: string(body)})
		case os.IsNotExist(err):
			// No user policy — common case, skip.
		default:
			// Permission denied, EIO, etc. — fail closed: an existing
			// policy that we can't load must not silently disappear.
			return nil, fmt.Errorf("read user policy %s: %w", userPath, err)
		}
	}

	for _, repoFile := range walkUp(opts.CWD, opts.Home) {
		body, err := os.ReadFile(repoFile)
		if err != nil {
			if os.IsNotExist(err) {
				// walkUp's os.Stat saw the file but it disappeared
				// before we read it — race with deletion is fine.
				continue
			}
			// Real I/O error (permission, EIO, etc.) — fail closed.
			return nil, fmt.Errorf("read repo policy %s: %w", repoFile, err)
		}
		repoPath := filepath.Dir(repoFile)
		trusted := false
		if opts.IsTrusted != nil {
			trusted = opts.IsTrusted(repoPath)
		}
		mod := Module{
			Layer:   LayerRepo,
			File:    repoFile,
			Body:    string(body),
			Trusted: trusted,
		}
		// If untrusted AND the file declares allow_override, warn once.
		if !trusted && declaresAllowOverride(string(body)) {
			emitUntrustedWarning(opts.WarnUntrusted, repoPath, repoFile)
		}
		out = append(out, mod)
	}

	return out, nil
}

func walkUp(cwd, home string) []string {
	var hits []string
	if cwd == "" {
		return nil
	}
	dir := cwd
	homeAbs := home
	for {
		candidate := filepath.Join(dir, ".failsafe.rego")
		if dir != homeAbs {
			if _, err := os.Stat(candidate); err == nil {
				hits = append(hits, candidate)
			}
		}
		if dir == homeAbs {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return hits
}

// declaresAllowOverride reports whether a Rego module body has any rule head
// named "allow_override". Cheap AST parse; tolerant of malformed input
// (returns false on parse error so the load can proceed).
func declaresAllowOverride(body string) bool {
	parsed, err := ast.ParseModule("", body)
	if err != nil {
		return false
	}
	for _, r := range parsed.Rules {
		if string(r.Head.Name) == "allow_override" {
			return true
		}
	}
	return false
}

func emitUntrustedWarning(cb func(repoPath, file string), repoPath, file string) {
	if cb != nil {
		cb(repoPath, file)
		return
	}
	// Default: write to stderr. Prefix matches spec §3.7's text.
	fmt.Fprintf(os.Stderr,
		"failsafe: repo %s ships allow_override rules but is untrusted; "+
			"overrides ignored. Run `failsafe trust .` to enable. (file: %s)\n",
		repoPath, redactHome(file),
	)
}

func redactHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
