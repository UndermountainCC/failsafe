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
// Canonical values ("enabled" / "disabled") are what get written to the mode
// file; the fact builder derives the boolean input.failsafe_enabled (which the
// bundled policies gate on via `not input.failsafe_enabled == false`) and the
// legacy input.mode string from them. Aliases are resolved here at the CLI
// boundary and never leak into the file or the policy layer.
func normalizeMode(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "enabled", "enable", "on", "closed", "close", "lock", "ro", "r", "read", "safe":
		return "enabled", true
	case "disabled", "disable", "off", "open", "unlock", "rw", "w", "write", "sudo":
		return "disabled", true
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
		fmt.Fprintf(out, "invalid mode %q (use 'enabled'/'disabled'; aliases: on/off, ro/rw, read/write, lock/sudo)\n", val)
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
