// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"fmt"
	"io"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/UndermountainCC/failsafe/internal/policy"
)

type AuditOptions struct {
	Home string
}

// Audit prints the effective policy chain at path, including every block rule
// and (loudly) every allow_override rule with file:line.
func Audit(path string, out io.Writer, opts AuditOptions) int {
	mods, err := policy.Discover(policy.DiscoverOpts{
		BundledLoader: loadBundledPolicies,
		Home:          opts.Home,
		CWD:           path,
	})
	if err != nil {
		fmt.Fprintf(out, "discover: %v\n", err)
		return 2
	}

	fmt.Fprintf(out, "Policy layers in effect at %s:\n\n", path)
	totalBlock, totalOverride := 0, 0

	bucketed := map[policy.Layer][]policy.Module{}
	for _, m := range mods {
		bucketed[m.Layer] = append(bucketed[m.Layer], m)
	}
	for _, layer := range []policy.Layer{policy.LayerBundled, policy.LayerUser, policy.LayerRepo} {
		group := bucketed[layer]
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(out, "  [%s]\n", layer)
		for _, m := range group {
			block, override, err := summarize(m)
			if err != nil {
				fmt.Fprintf(out, "    %s: parse error: %v\n", m.File, err)
				continue
			}
			fmt.Fprintf(out, "    %s — %d block rule(s)\n", m.File, len(block))
			totalBlock += len(block)
			for _, r := range override {
				fmt.Fprintf(out, "      ⚠  allow_override at %s:%d — %q\n", m.File, r.line, r.reason)
				totalOverride++
			}
		}
	}
	fmt.Fprintf(out, "\n%d block rules total, %d allow_override.\n", totalBlock, totalOverride)
	if totalOverride > 0 {
		fmt.Fprintln(out, "Run `failsafe explain <command>` to dry-run specific commands.")
	}
	return 0
}

type ruleSummary struct {
	line   int
	reason string
}

func summarize(m policy.Module) ([]ruleSummary, []ruleSummary, error) {
	mod, err := ast.ParseModule(m.File, m.Body)
	if err != nil {
		return nil, nil, err
	}
	var block, override []ruleSummary
	for _, r := range mod.Rules {
		s := ruleSummary{line: r.Location.Row, reason: extractReason(r)}
		switch string(r.Head.Name) {
		case "block":
			block = append(block, s)
		case "allow_override":
			override = append(override, s)
		}
	}
	return block, override, nil
}

func extractReason(r *ast.Rule) string {
	// Try head.Value, then head.Key (partial set rules).
	for _, t := range []*ast.Term{r.Head.Value, r.Head.Key} {
		if t == nil {
			continue
		}
		if obj, ok := t.Value.(ast.Object); ok {
			var found string
			obj.Foreach(func(k, v *ast.Term) {
				if s, isStr := k.Value.(ast.String); isStr && string(s) == "reason" {
					found = strings.Trim(v.String(), `"`)
				}
			})
			if found != "" {
				return found
			}
		}
	}
	return "(no static reason)"
}
