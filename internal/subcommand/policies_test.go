// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"strings"
	"testing"
)

func TestPoliciesList_ShowsBundled(t *testing.T) {
	var out bytes.Buffer
	code := PoliciesList(&out, PoliciesListOptions{Home: t.TempDir()})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	for _, want := range []string{"kubectl.rego", "helm.rego", "terraform.rego", "aws.rego", "git.rego"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in output:\n%s", want, out.String())
		}
	}
}
