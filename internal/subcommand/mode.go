// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"fmt"
	"io"
	"strings"

	"github.com/UndermountainCC/failsafe/internal/mode"
)

// normalizeMode maps user-friendly aliases to the two canonical mode values.
// Canonical values ("read" / "read & write") are what get written to the mode
// file and what the bundled Rego policies match on (input.mode == "read"), so
// aliases are resolved here at the CLI boundary and never leak into the file or
// the policy layer. Accepts: ro/r/read and rw/w/"read & write" (and a few
// punctuation variants), case-insensitive.
func normalizeMode(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ro", "r", "read":
		return "read", true
	case "rw", "w", "read & write", "read&write", "read+write", "readwrite":
		return "read & write", true
	default:
		return "", false
	}
}

type ModeOptions struct {
	Chain *mode.Chain
	Env   map[string]string
}

func ModeGet(out io.Writer, opts ModeOptions) int {
	val, src, err := opts.Chain.Resolve(opts.Env)
	if err != nil {
		fmt.Fprintf(out, "resolve: %v\n", err)
		return 1
	}
	if src == nil {
		fmt.Fprintf(out, "%s\t(default; no source resolved)\n", val)
		return 0
	}
	if fs, ok := src.(mode.FileSource); ok {
		path, _ := fs.Path(opts.Env)
		fmt.Fprintf(out, "%s\t(file: %s)\n", val, path)
		return 0
	}
	fmt.Fprintf(out, "%s\t(env)\n", val)
	return 0
}

func ModeSet(val string, out io.Writer, opts ModeOptions) int {
	canon, ok := normalizeMode(val)
	if !ok {
		fmt.Fprintf(out, "invalid mode %q (use 'rw' / 'ro', or 'read' / 'read & write')\n", val)
		return 2
	}
	_, path, ok := opts.Chain.FirstWritable(opts.Env)
	if !ok {
		fmt.Fprintln(out, "no writable mode source found")
		return 1
	}
	if err := atomicWrite(path, []byte(canon)); err != nil {
		fmt.Fprintf(out, "write %s: %v\n", path, err)
		return 1
	}
	fmt.Fprintf(out, "%s\t(file: %s)\n", canon, path)
	return 0
}
