// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/ast"
)

type ValidateOptions struct {
	Strict bool // promote warnings to failures
}

// Validate lints a .rego file: parse + package + rule-name + rule-shape checks.
func Validate(path string, out io.Writer, opts ValidateOptions) int {
	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(out, "✗ read %s: %v\n", path, err)
		return 2
	}
	mod, err := ast.ParseModule(path, string(body))
	if err != nil {
		fmt.Fprintf(out, "✗ parse error: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, "✓ parse OK")

	expectedSuffix := expectedPackage(path)
	gotPkg := mod.Package.Path.String()
	// Path strings are like "data.guard.repo" or "data.failsafe.repo"; trim "data."
	gotPkg = strings.TrimPrefix(gotPkg, "data.")
	// expectedSuffix == "" is the "skip strict check" sentinel for paths whose
	// layer can't be inferred from the filename alone (e.g., user-managed
	// example files at arbitrary paths). In that case we infer the layer
	// from the package declaration itself.
	// When expectedSuffix is non-empty ("repo" or "user"), we compare the
	// suffix of the declared package — this accepts BOTH guard.repo and
	// failsafe.repo (dual-namespace support).
	if expectedSuffix != "" && layerSuffix(gotPkg) != expectedSuffix {
		fmt.Fprintf(out, "✗ package: got %q, want suffix %q (e.g. guard.%s or failsafe.%s, based on filename)\n",
			gotPkg, expectedSuffix, expectedSuffix, expectedSuffix)
		return 1
	}
	fmt.Fprintf(out, "✓ package: %s\n", gotPkg)

	// allowedKey is the full declared package — used by forbiddenRuleNames /
	// summarizeAllowed which internally call layerSuffix / isRepoLayer.
	allowedKey := gotPkg
	// Reserved-rule check: only "block" is universally allowed; "allow_override"
	// is allowed only in the repo layer; helper rules (read_verbs, allowed_dry_run,
	// is_read_action, etc.) are FREE — Rego naturally namespaces helpers within
	// their package, so they can't conflict across layers. We only flag the
	// reserved names being used in a layer that doesn't permit them.
	forbidden := forbiddenRuleNames(allowedKey)
	for _, r := range mod.Rules {
		name := string(r.Head.Name)
		if forbidden[name] {
			fmt.Fprintf(out, "✗ rule %q is not permitted in layer %q (this layer is %s-only)\n",
				name, allowedKey, summarizeAllowed(allowedKey))
			return 1
		}
	}
	fmt.Fprintf(out, "✓ rule names: no reserved-rule violations\n")

	// Rule shape: every block / allow_override rule should produce {"reason": <string>}
	for _, r := range mod.Rules {
		name := string(r.Head.Name)
		if name != "block" && name != "allow_override" {
			continue
		}
		if !ruleProducesReasonObject(r) {
			fmt.Fprintf(out, "✗ rule %s at %s:%d: must produce object with \"reason\" key\n", name, path, r.Location.Row)
			return 1
		}
	}
	fmt.Fprintln(out, "✓ rule shapes: all block/allow_override return {\"reason\": ...}")

	// Best-effort fact-schema check: warn on `input.<unknown>` references.
	// Helps catch typos like `input.cmd` (correct: input.tool/verb/flags etc).
	// Warnings don't fail by default; --strict promotes them to failures.
	warnings := 0
	for _, ref := range collectInputRefs(mod) {
		if isKnownFactField(ref.field) {
			continue
		}
		fmt.Fprintf(out, "⚠ unknown fact field: input.%s (line %d) — not in fact schema; will be undefined at runtime\n",
			ref.field, ref.line)
		warnings++
	}
	if warnings == 0 {
		fmt.Fprintln(out, "✓ fact-field references: all known")
	}

	if opts.Strict && warnings > 0 {
		fmt.Fprintf(out, "\nFAIL (--strict): %d warning(s) treated as errors.\n", warnings)
		return 1
	}
	if warnings > 0 {
		fmt.Fprintf(out, "\nOK with %d warning(s).\n", warnings)
	} else {
		fmt.Fprintln(out, "\nOK.")
	}
	return 0
}

// knownFactFields enumerates the top-level keys of the Rego input fact
// (spec §5.1). Updated whenever the fact schema grows.
var knownFactFields = map[string]bool{
	"failsafe_enabled": true,
	"mode": true, "tool": true, "verb": true, "subverb": true,
	"flags": true, "positional": true, "env": true,
	"cwd": true, "now": true, "session": true, "raw": true,
	"git": true, "kubectl": true, // per-tool enrichment namespaces
}

func isKnownFactField(name string) bool { return knownFactFields[name] }

// inputRef captures a single `input.<field>` reference and its location.
type inputRef struct {
	field string
	line  int
}

// collectInputRefs walks the module AST and returns every Ref expression
// that starts with `input.<something>`. ast.Ref is a []*ast.Term, so the
// location lives on the individual terms; we read it from the head term.
func collectInputRefs(mod *ast.Module) []inputRef {
	var out []inputRef
	ast.WalkRefs(mod, func(r ast.Ref) bool {
		if len(r) < 2 {
			return false
		}
		v, ok := r[0].Value.(ast.Var)
		if !ok || string(v) != "input" {
			return false
		}
		field := refField(r)
		if field == "" {
			return false
		}
		line := 0
		if r[0].Location != nil {
			line = r[0].Location.Row
		} else if r[1].Location != nil {
			line = r[1].Location.Row
		}
		out = append(out, inputRef{field: field, line: line})
		return false
	})
	return out
}

// refField returns the second segment of an `input.<X>...` ref, or "".
// Handles both string-key form (`input["X"]`) and var-segment form (`input.X`).
func refField(ref ast.Ref) string {
	if len(ref) < 2 {
		return ""
	}
	if s, ok := ref[1].Value.(ast.String); ok {
		return string(s)
	}
	if v, ok := ref[1].Value.(ast.Var); ok {
		return string(v)
	}
	return ""
}

// layerSuffix strips a guard. or failsafe. namespace prefix, returning e.g.
// "repo" / "user" / "bundled.kubectl". Accepts both legacy and new namespace.
func layerSuffix(pkg string) string {
	for _, p := range []string{"failsafe.", "guard."} {
		if strings.HasPrefix(pkg, p) {
			return strings.TrimPrefix(pkg, p)
		}
	}
	return pkg
}

// isRepoLayer reports whether pkg is the repo layer in either namespace.
func isRepoLayer(pkg string) bool { return layerSuffix(pkg) == "repo" }

// expectedPackage returns the expected package path for well-known filenames,
// or "" when the filename gives no layer hint (skip strict equality check).
// For known filenames we accept EITHER the legacy guard.* OR the new failsafe.*
// namespace by returning "" and relying on layerSuffix in forbiddenRuleNames.
func expectedPackage(path string) string {
	base := filepath.Base(path)
	switch {
	case base == ".failsafe.rego":
		// Accept either guard.repo or failsafe.repo — return "" and rely on
		// layerSuffix to classify. But we still need to reject wrong.name, so
		// we use the suffix check in the caller rather than strict equality.
		return "repo" // sentinel: caller will compare layerSuffix(gotPkg) == expectedPackage
	case base == "policy.rego" && strings.Contains(path, "/.config/failsafe/"):
		return "user" // same sentinel approach
	default:
		// bundled: package name should be {guard,failsafe}.bundled.<tool>;
		// we don't validate the <tool> part here, just accept either prefix.
		return "" // empty means "skip strict check"
	}
}

// forbiddenRuleNames returns the set of RESERVED rule names that this layer
// must NOT declare. Helper rules (anything not in this set) are always free.
//   - repo layer (guard.repo or failsafe.repo): nothing reserved
//   - user / bundled / unknown: allow_override is reserved
func forbiddenRuleNames(pkg string) map[string]bool {
	if isRepoLayer(pkg) {
		return map[string]bool{}
	}
	return map[string]bool{"allow_override": true}
}

func summarizeAllowed(pkg string) string {
	if isRepoLayer(pkg) {
		return "block + allow_override"
	}
	return "block"
}

// ruleProducesReasonObject checks that a rule head produces an object with a
// "reason" key. The spec uses the partial set form (`block contains {"reason":
// ...} if { ... }`), which puts the term in r.Head.Key. The complete form
// (`foo := {...} if { ... }`) puts it in r.Head.Value. Inspect whichever is
// populated; if neither is an object with "reason", the rule is malformed.
func ruleProducesReasonObject(r *ast.Rule) bool {
	var term *ast.Term
	switch {
	case r.Head.Key != nil:
		term = r.Head.Key
	case r.Head.Value != nil:
		term = r.Head.Value
	default:
		return false
	}
	obj, ok := term.Value.(ast.Object)
	if !ok {
		return false
	}
	found := false
	obj.Foreach(func(k, _ *ast.Term) {
		if s, isStr := k.Value.(ast.String); isStr && string(s) == "reason" {
			found = true
		}
	})
	return found
}
