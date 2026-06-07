// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/UndermountainCC/failsafe/internal/mode"
)

type ToggleOptions struct {
	Chain *mode.Chain
	Env   map[string]string
}

// Toggle flips the first writable mode source between "enabled" and "disabled".
// Atomic via temp-file + rename.
func Toggle(out io.Writer, opts ToggleOptions) int {
	current, _, _ := opts.Chain.Resolve(opts.Env)
	next := "disabled"
	if current == "disabled" {
		next = "enabled"
	}
	_, path, ok := opts.Chain.FirstWritable(opts.Env)
	if !ok {
		fmt.Fprintln(out, "no writable mode source found")
		return 1
	}
	if err := atomicWrite(path, []byte(next)); err != nil {
		fmt.Fprintf(out, "write %s: %v\n", path, err)
		return 1
	}
	fmt.Fprintf(out, "%s → %s (%s)\n", current, next, path)
	return 0
}

func atomicWrite(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
