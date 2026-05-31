// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parityRun mirrors the original bash guard's test setup: it creates a fake HOME,
// writes the WezTerm pane-mode file, and invokes Hook with a Claude Code-shaped
// JSON payload. Returns (block, reason). Mode is the literal string written
// to the pane-mode file (e.g. "read" or "read & write").
func parityRun(t *testing.T, mode, command string, opts HookOptions) (block bool, reason string) {
	t.Helper()
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".claude", "pane-mode"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Reset env vars the mode chain reads — FAILSAFE_MODE in the runner's
	// environment would otherwise short-circuit the file-based source we're
	// trying to test, making every parity test silently pass.
	t.Setenv("FAILSAFE_MODE", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("ITERM_SESSION_ID", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	// Write the mode file as the WezTerm-style source.
	t.Setenv("WEZTERM_PANE", "42")
	if err := os.WriteFile(filepath.Join(home, ".claude", "pane-mode", "42"), []byte(mode), 0o644); err != nil {
		t.Fatal(err)
	}

	opts.Home = home
	in := strings.NewReader(fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":%q},"cwd":%q}`, command, home))
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, opts)
	if code != 0 {
		t.Fatalf("hook exit=%d, stderr=%s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		return false, ""
	}
	var d struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		// Could be allow_override JSON; treat as allow.
		return false, ""
	}
	return d.Decision == "block", d.Reason
}

// ──────────────────────────────────────────────────────────────────
// read mode (no mode file → defaults to read)
// ──────────────────────────────────────────────────────────────────

func TestParity_KubectlReadAllowed(t *testing.T) {
	allow := []string{
		"kubectl get pods",
		"kubectl describe pod foo",
		"kubectl logs foo",
		"kubectl exec -it foo -- bash",
		"kubectl port-forward svc/foo 8080",
	}
	for _, cmd := range allow {
		t.Run(cmd, func(t *testing.T) {
			b, r := parityRun(t, "read", cmd, HookOptions{})
			if b {
				t.Errorf("expected allow, blocked: %s", r)
			}
		})
	}
}

func TestParity_KubectlMutationsBlocked(t *testing.T) {
	block := []string{
		"kubectl apply -f foo.yaml",
		"kubectl delete pod foo",
		"kubectl scale deploy foo --replicas=0",
		"kubectl patch deploy foo -p '{}'",
		"kubectl drain node-1",
		"kubectl cordon node-1",
	}
	for _, cmd := range block {
		t.Run(cmd, func(t *testing.T) {
			got, _ := parityRun(t, "read", cmd, HookOptions{})
			if !got {
				t.Errorf("expected block")
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// command variants (absolute path, env prefix, command wrapper)
// ──────────────────────────────────────────────────────────────────

func TestParity_CommandVariants(t *testing.T) {
	cases := []struct {
		cmd   string
		block bool
	}{
		{"/usr/local/bin/kubectl apply -f x.yaml", true},
		{"KUBECONFIG=/tmp/foo kubectl apply -f x.yaml", true},
		{"command kubectl delete pod foo", true},
		{"/usr/local/bin/kubectl get pods", false},
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			got, r := parityRun(t, "read", tc.cmd, HookOptions{})
			if got != tc.block {
				t.Errorf("expected block=%v, got block=%v reason=%q", tc.block, got, r)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// chained commands (&&, |)
// ──────────────────────────────────────────────────────────────────

func TestParity_ChainedCommands(t *testing.T) {
	cases := []struct {
		cmd   string
		block bool
	}{
		{"echo yes && kubectl apply -f foo.yaml", true},
		{"echo yes | kubectl apply -f -", true},
		{"echo yes && kubectl get pods", false},
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			got, r := parityRun(t, "read", tc.cmd, HookOptions{})
			if got != tc.block {
				t.Errorf("expected block=%v, got block=%v reason=%q", tc.block, got, r)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// terraform
// ──────────────────────────────────────────────────────────────────

func TestParity_TerraformAllowed(t *testing.T) {
	allow := []string{
		"terraform plan",
		"terraform show",
		"terraform state list",
		"terraform state show foo",
		"terraform validate",
	}
	for _, cmd := range allow {
		t.Run(cmd, func(t *testing.T) {
			b, r := parityRun(t, "read", cmd, HookOptions{})
			if b {
				t.Errorf("expected allow, blocked: %s", r)
			}
		})
	}
}

func TestParity_TerraformBlocked(t *testing.T) {
	block := []string{
		"terraform apply",
		"terraform destroy",
		"terraform import foo bar",
	}
	for _, cmd := range block {
		t.Run(cmd, func(t *testing.T) {
			got, _ := parityRun(t, "read", cmd, HookOptions{})
			if !got {
				t.Errorf("expected block")
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// helm
// ──────────────────────────────────────────────────────────────────

func TestParity_HelmAllowed(t *testing.T) {
	allow := []string{
		"helm list",
		"helm get values foo",
		"helm status foo",
		"helm repo list",
	}
	for _, cmd := range allow {
		t.Run(cmd, func(t *testing.T) {
			b, r := parityRun(t, "read", cmd, HookOptions{})
			if b {
				t.Errorf("expected allow, blocked: %s", r)
			}
		})
	}
}

func TestParity_HelmBlocked(t *testing.T) {
	block := []string{
		"helm install foo bar",
		"helm upgrade foo bar",
		"helm uninstall foo",
	}
	for _, cmd := range block {
		t.Run(cmd, func(t *testing.T) {
			got, _ := parityRun(t, "read", cmd, HookOptions{})
			if !got {
				t.Errorf("expected block")
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// aws
// ──────────────────────────────────────────────────────────────────

func TestParity_AwsAllowed(t *testing.T) {
	allow := []string{
		"aws sts get-caller-identity",
		"aws s3 ls",
		"aws eks describe-cluster --name foo",
		"aws ecr describe-repositories",
		"aws iam list-roles",
		"aws s3api list-buckets",
	}
	for _, cmd := range allow {
		t.Run(cmd, func(t *testing.T) {
			b, r := parityRun(t, "read", cmd, HookOptions{})
			if b {
				t.Errorf("expected allow, blocked: %s", r)
			}
		})
	}
}

func TestParity_AwsBlocked(t *testing.T) {
	block := []string{
		"aws s3 rm s3://bucket/key",
		"aws ec2 terminate-instances --instance-ids i-123",
		"aws eks delete-cluster --name foo",
		"aws s3api put-object --bucket b --key k",
	}
	for _, cmd := range block {
		t.Run(cmd, func(t *testing.T) {
			got, _ := parityRun(t, "read", cmd, HookOptions{})
			if !got {
				t.Errorf("expected block")
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// read & write mode (write mode allows everything)
// ──────────────────────────────────────────────────────────────────

func TestParity_ReadWriteAllowsAll(t *testing.T) {
	allow := []string{
		"kubectl apply -f foo.yaml",
		"terraform destroy",
		"helm install foo bar",
		"aws ec2 terminate-instances --instance-ids i-123",
	}
	for _, cmd := range allow {
		t.Run(cmd, func(t *testing.T) {
			b, r := parityRun(t, "read & write", cmd, HookOptions{})
			if b {
				t.Errorf("expected allow in rw mode, blocked: %s", r)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// WEZTERM_PANE unset (defaults to read; no mode file resolves)
// ──────────────────────────────────────────────────────────────────

func TestParity_WezTermPaneUnset(t *testing.T) {
	cases := []struct {
		cmd   string
		block bool
	}{
		{"kubectl apply -f foo.yaml", true},
		{"kubectl get pods", false},
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			dir := t.TempDir()
			home := filepath.Join(dir, "home")
			if err := os.MkdirAll(home, 0o755); err != nil {
				t.Fatal(err)
			}
			// Clear all mode chain env vars so we fall through to default "read".
			t.Setenv("FAILSAFE_MODE", "")
			t.Setenv("WEZTERM_PANE", "")
			t.Setenv("TMUX_PANE", "")
			t.Setenv("ITERM_SESSION_ID", "")
			t.Setenv("KITTY_WINDOW_ID", "")
			t.Setenv("CLAUDE_SESSION_ID", "")

			in := strings.NewReader(fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":%q},"cwd":%q}`, tc.cmd, home))
			var stdout, stderr bytes.Buffer
			code := Hook(in, &stdout, &stderr, HookOptions{Home: home})
			if code != 0 {
				t.Fatalf("hook exit=%d, stderr=%s", code, stderr.String())
			}
			block := false
			var reason string
			if stdout.Len() > 0 {
				var d struct {
					Decision string `json:"decision"`
					Reason   string `json:"reason"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &d); err == nil {
					block = d.Decision == "block"
					reason = d.Reason
				}
			}
			if block != tc.block {
				t.Errorf("expected block=%v, got block=%v reason=%q", tc.block, block, reason)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// non-infra passthrough — git/npm/go/ls/cat aren't blocked
// (git IS in the registry via bundled YAML, but its bundled rego is empty)
// ──────────────────────────────────────────────────────────────────

func TestParity_NonInfraPassthrough(t *testing.T) {
	allow := []string{
		"git push origin main",
		"npm install",
		"go test ./...",
		"ls -la",
		"cat /etc/hosts",
	}
	for _, cmd := range allow {
		t.Run(cmd, func(t *testing.T) {
			b, r := parityRun(t, "read", cmd, HookOptions{})
			if b {
				t.Errorf("expected allow, blocked: %s", r)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// flags before subcommand — flag-positioned-before-verb cases
// ──────────────────────────────────────────────────────────────────

func TestParity_FlagBeforeVerb_NowAllowed(t *testing.T) {
	allow := []string{
		"kubectl --context arn:aws:eks:us-west-2:123456789012:cluster/dev get pods",
		"kubectl --context=arn:aws:eks:us-west-2:123456789012:cluster/dev get pods",
		"kubectl -n kube-system get pods",
		"kubectl --namespace kube-system get pods",
		"kubectl --context arn:aws:eks:us-west-2:123456789012:cluster/dev --namespace ns get pods",
		"kubectl -v=8 describe pod foo",
		"terraform -chdir=modules/foo plan",
		"terraform -chdir modules/foo plan",
		"helm --namespace kube-system list",
		"helm -n kube-system status foo",
		"aws --region us-west-2 s3 ls",
		"aws --profile dev sts get-caller-identity",
		"aws --region us-west-2 eks describe-cluster --name foo",
		"KUBECONFIG=/tmp/foo kubectl --context arn:aws:eks:us-west-2:123456789012:cluster/dev get pods",
	}
	for _, cmd := range allow {
		t.Run(cmd, func(t *testing.T) {
			got, r := parityRun(t, "read", cmd, HookOptions{})
			if got {
				t.Errorf("expected allow, blocked: %s", r)
			}
		})
	}
}

func TestParity_FlagBeforeVerb_StillBlocked(t *testing.T) {
	block := []string{
		"kubectl --context arn:aws:eks:us-west-2:123456789012:cluster/dev apply -f foo.yaml",
		"kubectl --context=arn:aws:eks:us-west-2:123456789012:cluster/dev delete pod foo",
		"kubectl -n ns delete pod foo",
		"terraform -chdir=modules/foo apply",
		"terraform -chdir=modules/foo destroy",
		"helm --namespace kube-system install foo bar",
		"helm -n kube-system uninstall foo",
		"aws --region us-west-2 s3 rm s3://bucket/key",
		"aws --region us-west-2 ec2 terminate-instances --instance-ids i-123",
		"KUBECONFIG=/tmp/foo kubectl --context arn:aws:eks:us-west-2:123456789012:cluster/dev apply -f x.yaml",
	}
	for _, cmd := range block {
		t.Run(cmd, func(t *testing.T) {
			got, _ := parityRun(t, "read", cmd, HookOptions{})
			if !got {
				t.Errorf("expected block")
			}
		})
	}
}
