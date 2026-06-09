// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package config

import "os"

// _osGetenv is the real os.Getenv call, referenced by realGetenv in config.go.
// Keeping it in a separate file makes the dependency on os explicit and the
// rest of config.go fully testable with injected env functions.
var _osGetenv = os.Getenv

// realEnviron returns os.Environ() as the default environ source for the
// injectable env provider.
func realEnviron() []string { return os.Environ() }
