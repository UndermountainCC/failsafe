// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

// Module is one Rego file plus its provenance.
type Module struct {
	Layer   Layer
	File    string
	Body    string
	Trusted bool // only meaningful for LayerRepo; bundled/user always treated as authoritative
}

// Engine holds compiled per-module queries plus enough state to apply
// reason precedence at evaluate time.
type Engine struct {
	cwd     string
	modules []compiledModule
}

type compiledModule struct {
	src         Module
	pkg         string // e.g. "data.guard.bundled.kubectl"
	blockQ      rego.PreparedEvalQuery
	overrideQ   rego.PreparedEvalQuery // zero-value when overrides are filtered
	hasOverride bool                   // false → don't evaluate overrideQ
	// firstLineByRule maps rule head name ("block", "allow_override", etc.)
	// to the source line of the first such rule in the module. Used by
	// evalAndTag to populate RuleHit.Line. v1 limitation: when a module
	// declares N rules with the same name, every hit gets the FIRST rule's
	// line. Sufficient for most policies (one rule per concept); imprecise
	// when a single module has multiple block rules that fire together.
	// A future v2 enhancement would use OPA's tracer for per-firing line.
	firstLineByRule map[string]int
}

// New compiles each module separately and prepares per-package queries so
// every rule firing can be tagged with (layer, file, line). It enforces
// layer permissions:
//   - LayerBundled / LayerUser: rejects modules that declare allow_override.
//   - LayerRepo with Trusted=false: drops allow_override rules at compile.
//   - LayerRepo with Trusted=true: keeps both block and allow_override.
func New(cwd string, mods []Module) (*Engine, error) {
	e := &Engine{cwd: cwd}
	ctx := context.Background()

	for _, m := range mods {
		parsed, err := ast.ParseModule(m.File, m.Body)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", m.File, err)
		}

		hasOverride := moduleHasOverride(parsed)

		if m.Layer != LayerRepo && hasOverride {
			return nil, fmt.Errorf(
				"%s: layer %s may not declare allow_override (repo layer only)",
				m.File, m.Layer,
			)
		}

		// Untrusted repo: strip allow_override rules from the AST before re-emitting.
		body := m.Body
		if m.Layer == LayerRepo && !m.Trusted && hasOverride {
			body = stripOverrideRules(parsed)
			hasOverride = false
		}

		pkg := strings.TrimPrefix(parsed.Package.Path.String(), "data.")
		dataPath := "data." + pkg

		blockQ, err := rego.New(
			rego.Query(dataPath+".block"),
			rego.Module(m.File, body),
		).PrepareForEval(ctx)
		if err != nil {
			return nil, fmt.Errorf("compile block %s: %w", m.File, err)
		}

		// Build the rule-name → first-line map from the (possibly stripped)
		// AST. The body fed to rego.Module is `body`; re-parse to get
		// locations consistent with what rego compiled.
		linesParsed, err := ast.ParseModule(m.File, body)
		if err != nil {
			return nil, fmt.Errorf("re-parse %s: %w", m.File, err)
		}
		firstLines := map[string]int{}
		for _, r := range linesParsed.Rules {
			n := string(r.Head.Name)
			if _, seen := firstLines[n]; seen {
				continue
			}
			if r.Location != nil {
				firstLines[n] = r.Location.Row
			}
		}

		cm := compiledModule{
			src: m, pkg: pkg, blockQ: blockQ,
			hasOverride: hasOverride, firstLineByRule: firstLines,
		}

		if hasOverride {
			overrideQ, err := rego.New(
				rego.Query(dataPath+".allow_override"),
				rego.Module(m.File, body),
			).PrepareForEval(ctx)
			if err != nil {
				return nil, fmt.Errorf("compile allow_override %s: %w", m.File, err)
			}
			cm.overrideQ = overrideQ
		}

		e.modules = append(e.modules, cm)
	}
	return e, nil
}

// Evaluate evaluates each module's queries against fact, collects every
// firing as a RuleHit, sorts by closest-to-cwd-then-layer, and combines
// per spec §4.2.
func (e *Engine) Evaluate(ctx context.Context, fact map[string]any) (Decision, error) {
	var blocks, overrides []RuleHit
	for _, m := range e.modules {
		bs, err := evalAndTag(ctx, m.blockQ, m.src, "block", m.firstLineByRule["block"], fact)
		if err != nil {
			return Decision{}, fmt.Errorf("block eval %s: %w", m.src.File, err)
		}
		blocks = append(blocks, bs...)
		if !m.hasOverride {
			continue
		}
		os, err := evalAndTag(ctx, m.overrideQ, m.src, "allow_override", m.firstLineByRule["allow_override"], fact)
		if err != nil {
			return Decision{}, fmt.Errorf("allow_override eval %s: %w", m.src.File, err)
		}
		overrides = append(overrides, os...)
	}

	sortByClosestThenLayer(blocks, e.cwd)
	sortByClosestThenLayer(overrides, e.cwd)

	// Filter overrides for the decision: malformed override rules MUST
	// NOT unblock — that's fail-open. The malformed hits stay in the
	// trace so audit/explain shows the bug, but they don't influence the
	// block/allow decision. (Block hits, including malformed ones,
	// remain — fail-closed for blocks.)
	var validOverrides []RuleHit
	for _, o := range overrides {
		if !o.Malformed {
			validOverrides = append(validOverrides, o)
		}
	}

	// Fail-closed: a malformed block rule MUST still block, even when a
	// well-formed override matches. Otherwise a buggy block + a working
	// override silently allows the action. Surface the malformed-block
	// reason so the policy bug is visible to the user.
	for _, b := range blocks {
		if b.Malformed {
			return Decision{
				Block:  true,
				Reason: b.Reason,
				Trace:  append(blocks, overrides...),
			}, nil
		}
	}

	if len(validOverrides) > 0 {
		return Decision{
			Block:          false,
			OverrideReason: validOverrides[0].Reason,
			Trace:          append(blocks, overrides...), // include malformed for visibility
		}, nil
	}
	if len(blocks) > 0 {
		return Decision{
			Block:  true,
			Reason: blocks[0].Reason,
			Trace:  append(blocks, overrides...),
		}, nil
	}
	return Decision{Block: false, Trace: append(blocks, overrides...)}, nil
}

// moduleHasOverride reports whether any rule head named "allow_override" exists.
func moduleHasOverride(parsed *ast.Module) bool {
	for _, r := range parsed.Rules {
		if string(r.Head.Name) == "allow_override" {
			return true
		}
	}
	return false
}

// stripOverrideRules removes allow_override rules from a parsed module
// and returns the resulting source. Used to filter untrusted-repo overrides
// at compile time.
func stripOverrideRules(parsed *ast.Module) string {
	kept := parsed.Rules[:0]
	for _, r := range parsed.Rules {
		if string(r.Head.Name) == "allow_override" {
			continue
		}
		kept = append(kept, r)
	}
	parsed.Rules = kept
	return parsed.String()
}

// evalAndTag runs the prepared query and returns one RuleHit per matched
// rule output, tagged with the module's provenance. firstLine populates
// RuleHit.Line — see compiledModule.firstLineByRule for limitations.
//
// FAIL-CLOSED on malformed output: when a rule fires but its output is
// not a {"reason": <string>} object, we still emit a RuleHit, with a
// synthetic reason naming the policy bug. For `block` rules this means a
// malformed rule still blocks (the user sees "policy bug: ..." rather
// than silently allowing). For `allow_override` it means a malformed
// override behaves as if it didn't fire (the synthetic reason is fine
// because the engine only consumes overrides[0].Reason on success).
func evalAndTag(ctx context.Context, q rego.PreparedEvalQuery, src Module, name string, firstLine int, fact map[string]any) ([]RuleHit, error) {
	rs, err := q.Eval(ctx, rego.EvalInput(fact))
	if err != nil {
		return nil, err
	}
	mkHit := func(reason string, malformed bool) RuleHit {
		return RuleHit{
			Layer: src.Layer, File: src.File, Line: firstLine,
			Name: name, Reason: reason, Malformed: malformed,
		}
	}
	bugReason := func(detail string) string {
		return "(policy bug in " + src.File + ": " + name + " rule fired with " + detail + ")"
	}

	var hits []RuleHit
	for _, r := range rs {
		for _, expr := range r.Expressions {
			items, ok := expr.Value.([]any)
			if !ok {
				if obj, ok := expr.Value.(map[string]any); ok {
					reason, ok := extractReason(obj)
					if !ok {
						hits = append(hits, mkHit(bugReason(`missing/empty/non-string "reason"`), true))
					} else {
						hits = append(hits, mkHit(reason, false))
					}
					continue
				}
				if expr.Value == nil {
					continue // no firings — normal case
				}
				hits = append(hits, mkHit(bugReason(
					fmt.Sprintf("non-object/non-set output (%T)", expr.Value),
				), true))
				continue
			}
			for _, item := range items {
				obj, ok := item.(map[string]any)
				if !ok {
					hits = append(hits, mkHit(bugReason(
						fmt.Sprintf("non-object set element (%T)", item),
					), true))
					continue
				}
				reason, ok := extractReason(obj)
				if !ok {
					hits = append(hits, mkHit(bugReason(`missing/empty/non-string "reason"`), true))
					continue
				}
				hits = append(hits, mkHit(reason, false))
			}
		}
	}
	return hits, nil
}

// extractReason returns the rule's "reason" string and ok=true on a
// well-formed object; ok=false if the field is missing, non-string, or empty.
func extractReason(obj map[string]any) (string, bool) {
	v, present := obj["reason"]
	if !present {
		return "", false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}

// sortByClosestThenLayer sorts so the most-specific-to-cwd rule wins
// reason precedence: closer repo file > shallower repo file > user > bundled.
func sortByClosestThenLayer(hits []RuleHit, cwd string) {
	sort.SliceStable(hits, func(i, j int) bool {
		di, dj := distance(hits[i], cwd), distance(hits[j], cwd)
		if di != dj {
			return di < dj
		}
		return layerRank(hits[i].Layer) < layerRank(hits[j].Layer)
	})
}

// distance is a numeric score: smaller = closer to cwd. Repo files use the
// negative of their path-depth match with cwd; user is a fixed mid value;
// bundled is the largest (furthest).
func distance(h RuleHit, cwd string) int {
	switch h.Layer {
	case LayerRepo:
		// Repo files closer to cwd have shorter path-distance. Use the
		// number of parent steps from cwd to the file's directory.
		fileDir := filepath.Dir(h.File)
		return parentSteps(cwd, fileDir)
	case LayerUser:
		return 10000 // fixed; further than any reasonable repo walk
	case LayerBundled:
		return 100000 // furthest
	}
	return 1000000
}

func parentSteps(cwd, target string) int {
	rel, err := filepath.Rel(target, cwd)
	if err != nil {
		return 9999
	}
	if rel == "." {
		return 0
	}
	steps := 0
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if seg == "" || seg == "." {
			continue
		}
		steps++
	}
	return steps
}

func layerRank(l Layer) int {
	switch l {
	case LayerRepo:
		return 0
	case LayerUser:
		return 1
	case LayerBundled:
		return 2
	}
	return 3
}
