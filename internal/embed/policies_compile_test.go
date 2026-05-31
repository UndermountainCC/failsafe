// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package embed

import (
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/rego"
)

// TestBundledRegoCompiles checks that every embedded .rego file parses and
// prepares cleanly under OPA. Lives in this package because it consumes
// the embed FS via ReadBundledPolicy + BundledPolicyNames.
func TestBundledRegoCompiles(t *testing.T) {
	ctx := context.Background()
	names := BundledPolicyNames()
	if len(names) != 6 {
		t.Fatalf("expected 6 bundled policies, got %d (%v)", len(names), names)
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".rego") {
			t.Errorf("non-rego in bundled policies: %s", name)
			continue
		}
		body, err := ReadBundledPolicy(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		_, err = rego.New(
			rego.Query("data"),
			rego.Module(name, string(body)),
		).PrepareForEval(ctx)
		if err != nil {
			t.Errorf("rego compile %s failed: %v", name, err)
		}
	}
}
