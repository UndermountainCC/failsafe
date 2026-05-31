// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package enrich

import (
	"context"
	"testing"
)

func TestKubectlContext_ARN(t *testing.T) {
	fact := map[string]any{
		"flags": map[string]any{
			"context": "arn:aws:eks:us-west-2:123456789012:cluster/prod",
		},
	}
	KubectlContextEnricher{}.Enrich(context.Background(), fact)
	k, _ := fact["kubectl"].(map[string]any)
	if k["cluster_name"] != "prod" {
		t.Errorf("cluster_name = %v", k["cluster_name"])
	}
	if k["current_context"] != "arn:aws:eks:us-west-2:123456789012:cluster/prod" {
		t.Errorf("current_context not preserved: %v", k["current_context"])
	}
}

func TestKubectlContext_NonARNLeavesClusterUnset(t *testing.T) {
	fact := map[string]any{
		"flags": map[string]any{"context": "minikube"},
	}
	KubectlContextEnricher{}.Enrich(context.Background(), fact)
	k, _ := fact["kubectl"].(map[string]any)
	if _, has := k["cluster_name"]; has {
		t.Error("cluster_name should not be set for non-ARN context")
	}
	if k["current_context"] != "minikube" {
		t.Errorf("current_context = %v", k["current_context"])
	}
}

func TestKubectlContext_GovCloudARN(t *testing.T) {
	fact := map[string]any{
		"flags": map[string]any{
			"context": "arn:aws-us-gov:eks:us-gov-west-1:123:cluster/secure",
		},
	}
	KubectlContextEnricher{}.Enrich(context.Background(), fact)
	k, _ := fact["kubectl"].(map[string]any)
	if k["cluster_name"] != "secure" {
		t.Errorf("cluster_name = %v, want 'secure'", k["cluster_name"])
	}
}

func TestKubectlContext_NoFlagIsNoop(t *testing.T) {
	fact := map[string]any{}
	KubectlContextEnricher{}.Enrich(context.Background(), fact)
	if _, has := fact["kubectl"]; has {
		t.Error("expected no kubectl key when --context absent")
	}
}
