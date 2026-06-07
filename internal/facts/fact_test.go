// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"testing"
	"time"

	"github.com/UndermountainCC/failsafe/internal/enrich"
	"github.com/UndermountainCC/failsafe/internal/tools"
)

func TestBuildEmitsBooleanAndLegacyMode(t *testing.T) {
	en := Builder{Mode: "enabled"}.Build()
	if en["failsafe_enabled"] != true || en["mode"] != "read" {
		t.Fatalf("enabled: failsafe_enabled=%v mode=%v", en["failsafe_enabled"], en["mode"])
	}
	dis := Builder{Mode: "disabled"}.Build()
	if dis["failsafe_enabled"] != false || dis["mode"] != "read & write" {
		t.Fatalf("disabled: failsafe_enabled=%v mode=%v", dis["failsafe_enabled"], dis["mode"])
	}
}

func TestBuild_Basic(t *testing.T) {
	parsed := tools.Parsed{
		Verb:       "apply",
		Flags:      map[string]interface{}{"context": "arn:aws:eks:us-west-2:1:cluster/prod"},
		Positional: []string{"-f", "manifests/"},
	}
	enrichReg := enrich.NewRegistry()
	enrichReg.Register(enrich.KubectlContextEnricher{})
	b := Builder{
		Mode:          "read",
		Tool:          "kubectl",
		CWD:           "/tmp/x",
		Now:           time.Date(2026, 4, 28, 13, 42, 0, 0, time.UTC),
		Parsed:        parsed,
		EnricherNames: []string{"kubectl_context"},
		Enrichers:     enrichReg,
		Raw:           "kubectl --context arn:aws:eks:us-west-2:1:cluster/prod apply -f manifests/",
	}
	fact := b.Build()
	if fact["mode"] != "read" {
		t.Errorf("mode = %v", fact["mode"])
	}
	if fact["tool"] != "kubectl" {
		t.Errorf("tool = %v", fact["tool"])
	}
	if fact["verb"] != "apply" {
		t.Errorf("verb = %v", fact["verb"])
	}
	flags, _ := fact["flags"].(map[string]any)
	if flags["context"] != "arn:aws:eks:us-west-2:1:cluster/prod" {
		t.Errorf("flags.context = %v", flags["context"])
	}
	k, _ := fact["kubectl"].(map[string]any)
	if k["cluster_name"] != "prod" {
		t.Errorf("kubectl.cluster_name = %v", k["cluster_name"])
	}
	if fact["cwd"] != "/tmp/x" {
		t.Errorf("cwd = %v", fact["cwd"])
	}
	if fact["now"] != "2026-04-28T13:42:00Z" {
		t.Errorf("now = %v", fact["now"])
	}
}
