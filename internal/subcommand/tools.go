// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	embedfs "github.com/UndermountainCC/failsafe/internal/embed"
)

type ToolsListOptions struct {
	Home     string
	ToolsDir string // explicit tools directory; empty → derive from Home
}

func ToolsList(out io.Writer, opts ToolsListOptions) int {
	type row struct{ name, source string }
	rows := []row{
		{"kubectl", "built-in Go"},
		{"helm", "built-in Go"},
	}
	for _, n := range embedfs.BundledToolNames() {
		rows = append(rows, row{strings.TrimSuffix(n, ".yaml"), "bundled YAML"})
	}
	userDir := opts.ToolsDir
	if userDir == "" && opts.Home != "" {
		userDir = filepath.Join(opts.Home, ".config", "failsafe", "tools")
	}
	if userDir != "" {
		entries, _ := fs.ReadDir(os.DirFS(userDir), ".")
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			rows = append(rows, row{strings.TrimSuffix(e.Name(), ".yaml"), "user YAML at " + filepath.Join(userDir, e.Name())})
		}
	}
	for _, r := range rows {
		fmt.Fprintf(out, "%-12s  (%s)\n", r.name, r.source)
	}
	return 0
}
