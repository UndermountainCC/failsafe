// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"fmt"
	"io"

	embedfs "github.com/UndermountainCC/failsafe/internal/embed"
	"github.com/UndermountainCC/failsafe/internal/policy"
)

type PoliciesListOptions struct {
	Home string
	CWD  string
}

func PoliciesList(out io.Writer, opts PoliciesListOptions) int {
	for _, n := range embedfs.BundledPolicyNames() {
		fmt.Fprintf(out, "[bundled] %s\n", n)
	}
	if opts.Home == "" {
		return 0
	}
	mods, _ := policy.Discover(policy.DiscoverOpts{
		BundledLoader: func() ([]policy.Module, error) { return nil, nil },
		Home:          opts.Home,
		CWD:           opts.CWD,
	})
	for _, m := range mods {
		switch m.Layer {
		case policy.LayerUser:
			fmt.Fprintf(out, "[user]    %s\n", m.File)
		case policy.LayerRepo:
			fmt.Fprintf(out, "[repo]    %s\n", m.File)
		}
	}
	return 0
}
