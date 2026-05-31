// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package embed exposes the bundled tool YAMLs and Rego policies that ship
// with the binary via Go's //go:embed directive.
package embed

import (
	"embed"
	"sort"
)

//go:embed all:tools all:policies
var bundled embed.FS

// BundledToolNames lists the YAML files under tools/ inside the binary.
func BundledToolNames() []string { return list("tools") }

// BundledPolicyNames lists the .rego files under policies/.
func BundledPolicyNames() []string { return list("policies") }

func list(dir string) []string {
	entries, err := bundled.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// ReadBundledTool returns the YAML body for a bundled tool by name.
func ReadBundledTool(name string) ([]byte, error) {
	return bundled.ReadFile("tools/" + name)
}

// ReadBundledPolicy returns the Rego body for a bundled policy by name.
func ReadBundledPolicy(name string) ([]byte, error) {
	return bundled.ReadFile("policies/" + name)
}
