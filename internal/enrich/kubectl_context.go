// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package enrich

import (
	"context"
	"strings"
)

// KubectlContextEnricher populates fact.kubectl.{current_context, cluster_name}
// from the raw --context flag value captured in fact.flags.context. Conservative:
// cluster_name is only extracted when the value parses as an EKS ARN; otherwise
// only current_context is preserved (verbatim).
//
// Pure-CPU work — the ctx parameter is intentionally unused; the enricher
// completes in microseconds and has no IO to interrupt.
type KubectlContextEnricher struct{}

func (KubectlContextEnricher) Name() string { return "kubectl_context" }

func (KubectlContextEnricher) Enrich(_ context.Context, fact map[string]any) {
	flags, _ := fact["flags"].(map[string]any)
	if flags == nil {
		return
	}
	ctx, _ := flags["context"].(string)
	if ctx == "" {
		return
	}
	out := map[string]any{"current_context": ctx}
	// Extract cluster name from EKS ARN: arn:<partition>:eks:REGION:ACCT:cluster/NAME
	// Covers commercial (aws), GovCloud (aws-us-gov), and China (aws-cn) partitions.
	for _, prefix := range []string{"arn:aws:eks:", "arn:aws-us-gov:eks:", "arn:aws-cn:eks:"} {
		if strings.HasPrefix(ctx, prefix) {
			if idx := strings.LastIndex(ctx, "cluster/"); idx != -1 {
				out["cluster_name"] = ctx[idx+len("cluster/"):]
			}
			break
		}
	}
	fact["kubectl"] = out
}
