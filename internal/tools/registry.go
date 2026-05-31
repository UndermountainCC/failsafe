// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import "github.com/UndermountainCC/failsafe/internal/shellparser"

// Registry holds Tool implementations indexed by registration order. The
// shell parser (internal/shellparser) does the heavy lifting of extracting
// EffectiveCalls from raw command strings, including unwrapping safe
// wrappers and skipping env-prefix tokens. The registry's only job is to
// look up the Tool for a given call's binary name.
type Registry struct {
	tools []Tool
}

func NewRegistry() *Registry { return &Registry{} }

// Add appends a Tool. Later-added tools win on Match() collision; see
// Find for collision semantics. The registry's caller (cmd/failsafe's
// main.go) controls insertion order: built-in Go tools first, bundled
// YAMLs next, user YAMLs last so user overrides win.
func (r *Registry) Add(t Tool) { r.tools = append(r.tools, t) }

// All returns registered tools in registration order. Used by `failsafe
// tools list`.
func (r *Registry) All() []Tool { return append([]Tool(nil), r.tools...) }

// Find returns the Tool matching the call's binary name, or (nil, false)
// if none matches. **Iterates in reverse registration order: last-added
// wins on Match() collision.** Spec §3.3 mandates "later wins" so user
// YAMLs (registered after bundled) override built-in Go tools and bundled
// YAMLs. Env-prefix vars and chain-ops are not the registry's concern —
// they're already resolved by the shell parser.
func (r *Registry) Find(call shellparser.EffectiveCall) (Tool, bool) {
	for i := len(r.tools) - 1; i >= 0; i-- {
		if r.tools[i].Match(call.Name) {
			return r.tools[i], true
		}
	}
	return nil, false
}
