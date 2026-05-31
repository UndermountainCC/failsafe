// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/UndermountainCC/failsafe/internal/trust"
)

// TrustOptions configures the Trust subcommand. Home and CWD are explicit so
// tests can drive the dispatcher without touching the process environment.
type TrustOptions struct {
	Home   string
	CWD    string
	Reason string // for `trust .` / `trust <path>`
}

// Trust dispatches based on args[0] (the verb): "list", "remove", "check", or
// a path/dot to add. Empty args means "add cwd's repo".
func Trust(args []string, out io.Writer, opts TrustOptions) int {
	if opts.Home == "" {
		opts.Home = os.Getenv("HOME")
	}
	if len(args) == 0 {
		// No args = "add cwd's repo"
		return trustAddDot(out, opts)
	}
	switch args[0] {
	case "list":
		return trustList(out, opts)
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(out, "usage: failsafe trust remove <path>")
			return 2
		}
		return trustRemove(args[1], out, opts)
	case "check":
		path := opts.CWD
		if len(args) >= 2 {
			path = args[1]
		}
		return trustCheck(path, out, opts)
	case ".":
		return trustAddDot(out, opts)
	default:
		// Treat as path to add.
		return trustAdd(args[0], out, opts)
	}
}

// findRepoRoot walks up from cwd looking for a .failsafe.rego, stopping at
// home (exclusive — same convention as policy chain). Returns "" if not found.
func findRepoRoot(cwd, home string) string {
	dir := cwd
	for {
		if dir == "" || dir == home {
			return ""
		}
		if _, err := os.Stat(filepath.Join(dir, ".failsafe.rego")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func trustAddDot(out io.Writer, opts TrustOptions) int {
	repo := findRepoRoot(opts.CWD, opts.Home)
	if repo == "" {
		fmt.Fprintf(out, "no .failsafe.rego found at or above %s; create one before running `failsafe trust .`\n", opts.CWD)
		return 1
	}
	return trustAdd(repo, out, opts)
}

func trustAdd(path string, out io.Writer, opts TrustOptions) int {
	tr, err := trust.Load(opts.Home)
	if err != nil {
		fmt.Fprintf(out, "trust load: %v\n", err)
		return 2
	}
	if err := tr.Add(path, opts.Reason); err != nil {
		if errors.Is(err, trust.ErrAlreadyTrusted) {
			fmt.Fprintf(out, "%s is already trusted\n", path)
			return 0
		}
		fmt.Fprintf(out, "add: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "trusted %s\n", path)
	return 0
}

func trustList(out io.Writer, opts TrustOptions) int {
	tr, err := trust.Load(opts.Home)
	if err != nil {
		fmt.Fprintf(out, "trust load: %v\n", err)
		return 2
	}
	repos := tr.List()
	if len(repos) == 0 {
		fmt.Fprintln(out, "no trusted repos.")
		return 0
	}
	for _, r := range repos {
		line := r.Path
		if r.Reason != "" {
			line += " — " + r.Reason
		}
		if r.AddedAt != "" {
			line += "  (added " + r.AddedAt + ")"
		}
		fmt.Fprintln(out, line)
	}
	return 0
}

func trustRemove(path string, out io.Writer, opts TrustOptions) int {
	tr, err := trust.Load(opts.Home)
	if err != nil {
		fmt.Fprintf(out, "trust load: %v\n", err)
		return 2
	}
	if err := tr.Remove(path); err != nil {
		if errors.Is(err, trust.ErrNotTrusted) {
			fmt.Fprintf(out, "%s was not trusted\n", path)
			return 1
		}
		fmt.Fprintf(out, "remove: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "removed %s\n", path)
	return 0
}

func trustCheck(path string, out io.Writer, opts TrustOptions) int {
	tr, err := trust.Load(opts.Home)
	if err != nil {
		fmt.Fprintf(out, "trust load: %v\n", err)
		return 2
	}
	// Default-cwd path may be the cwd itself OR the cwd's repo (if cwd is
	// inside a repo). Match the spec: check should examine the cwd's repo.
	// Find the repo root if path is the bare cwd; fall back to path itself.
	if path == opts.CWD {
		if root := findRepoRoot(path, opts.Home); root != "" {
			path = root
		}
	}
	if tr.IsTrusted(path) {
		fmt.Fprintf(out, "%s is trusted\n", path)
		return 0
	}
	fmt.Fprintf(out, "%s is NOT trusted\n", path)
	return 1
}
