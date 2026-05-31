// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import "path/filepath"

// NewFailsafeTool returns the Go-coded failsafe Tool. Bundled failsafe.rego
// (internal/embed/policies/failsafe.rego) keys off `input.verb` to block
// `toggle`, `hook`, and `mcp` from Claude — those are user-only or runtime-
// lifecycle subcommands. Registering the tool here is what wires failsafe
// commands into the policy pipeline; without the registry entry, Claude
// invocations of `failsafe ...` would skip evaluation entirely (registry
// `Find` returns no tool → call passes through unchecked).
//
// This file mirrors kubectl.go and helm.go: same Parse loop shape, same
// value-flag map, same subverb extraction pattern (helm's `helmSubverbsByVerb`
// — see Task 21 for the parity story behind that). failsafe exists as a
// real tool surface (cobra-style verbs), so the cobra-mirror parser shape
// fits cleanly with no special-cases.
func NewFailsafeTool() Tool { return &failsafeTool{} }

type failsafeTool struct{}

func (failsafeTool) Name() string          { return "failsafe" }
func (failsafeTool) Match(tok string) bool { return filepath.Base(tok) == "failsafe" }
func (failsafeTool) Enrichers() []string   { return nil }

// failsafeValueFlags is empty by design. failsafe's CLI surface is mostly
// boolean flags (`--version`, `--help`, `--strict`); none of them are
// policy-relevant, so we don't need to capture values. Keeping the map
// declared (rather than inline) preserves shape-parity with kubectl/helm and
// makes it cheap to add a value flag later.
var failsafeValueFlags = map[string]struct{}{}

// failsafeShortToLong is empty: failsafe doesn't define short forms today.
// Same shape-parity reasoning as the value-flag map.
var failsafeShortToLong = map[string]string{}

// failsafeSubverbsByVerb mirrors helm's verb→subverb whitelist. Bundled
// failsafe.rego doesn't currently key off subverb (it blocks whole verbs:
// toggle, hook, mcp), but extracting subverbs keeps the parsed fact accurate
// for repo/user policies that may want finer granularity (e.g. allow
// `mode get` but not `mode set` outside an opt-in path).
var failsafeSubverbsByVerb = map[string][]string{
	"mode":     {"get", "set"},
	"trust":    {"list", "remove", "check"},
	"tools":    {"list"},
	"policies": {"list"},
}

func (failsafeTool) Parse(args []string) (Parsed, error) {
	out := Parsed{Flags: map[string]interface{}{}}
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if len(a) >= 2 && a[:2] == "--" {
			name, val, hasEq := splitOnce(a[2:], "=")
			if _, isVal := failsafeValueFlags[name]; isVal {
				if hasEq {
					out.Flags[name] = val
				} else if i+1 < len(args) {
					out.Flags[name] = args[i+1]
					i++
				}
			} else {
				// Unknown long flag: stored as bool=true and we DON'T consume
				// the next token. Same conservative reading as kubectl/helm —
				// see kubectl.go's comment on the parallel branch.
				out.Flags[name] = true
			}
			i++
			continue
		}
		if len(a) >= 2 && a[0] == '-' {
			body := a[1:]
			if eq := indexByte(body, '='); eq != -1 {
				short := body[:eq]
				val := body[eq+1:]
				if long, ok := failsafeShortToLong[short]; ok {
					out.Flags[long] = val
				} else {
					out.Flags[short] = val
				}
				i++
				continue
			}
			if len(body) == 1 {
				if long, ok := failsafeShortToLong[body]; ok {
					if _, isVal := failsafeValueFlags[long]; isVal && i+1 < len(args) {
						out.Flags[long] = args[i+1]
						i += 2
						continue
					}
					out.Flags[long] = true
					i++
					continue
				}
				out.Flags[body] = true
				i++
				continue
			}
			out.Flags[body] = true
			i++
			continue
		}
		if out.Verb == "" {
			out.Verb = a
			i++
			// Look up subverbs for compound verbs like `failsafe mode get`.
			if subs, ok := failsafeSubverbsByVerb[a]; ok && i < len(args) {
				next := args[i]
				for _, sv := range subs {
					if next == sv {
						out.Subverb = next
						i++
						break
					}
				}
			}
			continue
		}
		out.Positional = append(out.Positional, a)
		i++
	}
	for ; i < len(args); i++ {
		out.Positional = append(out.Positional, args[i])
	}
	return out, nil
}
