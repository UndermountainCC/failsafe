// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

// splitOnce returns (head, tail, true) when sep is found in s, or
// (s, "", false) otherwise. It exists so production code (kubectl.go,
// helm.go, yamltool.go) and tests in this package can share one helper
// without dragging in stdlib imports for one-line operations.
func splitOnce(s, sep string) (string, string, bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return s, "", false
}

// indexByte is bytes.IndexByte but for strings without an extra import.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
