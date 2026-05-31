// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UndermountainCC/failsafe/internal/auditlog"
)

func TestHook_LogsBlockDecisionWithSession(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "42")
	var logbuf bytes.Buffer
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"kubectl delete pod x"},"cwd":"/tmp","session_id":"sess-1"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{
		Home:         t.TempDir(),
		ModeOverride: "read",
		Logger:       &auditlog.Logger{Writer: &logbuf},
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"decision":"block"`) {
		t.Fatalf("expected block on stdout; got %s", stdout.String())
	}
	log := logbuf.String()
	for _, want := range []string{
		`"decision":"block"`, `"tool":"kubectl"`, `"verb":"delete"`,
		`"agent_type":"claude-code"`, `"agent_session_id":"sess-1"`, `"terminal_pane":"42"`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %s\ngot: %s", want, log)
		}
	}
}

func TestHook_LogsAllowForRegisteredTool(t *testing.T) {
	var logbuf bytes.Buffer
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"kubectl get pods"},"cwd":"/tmp","session_id":"sess-2"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{
		Home:         t.TempDir(),
		ModeOverride: "read",
		Logger:       &auditlog.Logger{Writer: &logbuf},
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	log := logbuf.String()
	for _, want := range []string{`"decision":"allow"`, `"tool":"kubectl"`, `"verb":"get"`, `"agent_session_id":"sess-2"`} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %s\ngot: %s", want, log)
		}
	}
}

func TestHook_DoesNotLogNonInfraCommand(t *testing.T) {
	var logbuf bytes.Buffer
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"echo hello"},"cwd":"/tmp","session_id":"sess-3"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{
		Home:   t.TempDir(),
		Logger: &auditlog.Logger{Writer: &logbuf},
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if logbuf.Len() != 0 {
		t.Errorf("non-infra command must not be logged; got %q", logbuf.String())
	}
}

func TestHook_LogsRefuseAsBlock(t *testing.T) {
	var logbuf bytes.Buffer
	// eval → refuse (design §3.5: dynamic dispatch). A refused command was
	// still attempted, so the user wants it in the trail.
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"eval \"kubectl apply -f x.yaml\""},"cwd":"/tmp","session_id":"sess-4"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{
		Home:   t.TempDir(),
		Logger: &auditlog.Logger{Writer: &logbuf},
	})
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"decision":"block"`) {
		t.Fatalf("expected refuse block on stdout; got %s", stdout.String())
	}
	if !strings.Contains(logbuf.String(), `"decision":"block"`) || !strings.Contains(logbuf.String(), `"agent_session_id":"sess-4"`) {
		t.Errorf("expected refuse logged as block with session; got: %s", logbuf.String())
	}
}

func TestHook_AllowsByDefault(t *testing.T) {
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"echo hello"},"cwd":"/tmp"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{})
	if code != 0 {
		t.Errorf("exit = %d, stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty on allow, got %q", stdout.String())
	}
}

func TestHook_BadJSON(t *testing.T) {
	in := strings.NewReader(`not json`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{})
	if code == 0 {
		t.Errorf("expected non-zero exit on bad JSON; stdout=%q", stdout.String())
	}
}

// A malformed user tool YAML must fail closed: previously the hook
// silently skipped the bad file, leaving its tool's commands unguarded.
// Now buildRegistry returns an error and Hook surfaces it as a block.
func TestHook_MalformedUserToolBlocks(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	toolsDir := filepath.Join(home, ".config", "failsafe", "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Malformed YAML — missing required `match` field.
	if err := os.WriteFile(filepath.Join(toolsDir, "broken.yaml"),
		[]byte("name: broken\n# no match field\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"echo hi"},"cwd":"/tmp"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{Home: home})
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "failsafe cannot start") {
		t.Errorf("expected fail-closed block with 'failsafe cannot start' reason; got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "broken.yaml") {
		t.Errorf("expected error to mention broken.yaml; got: %s", stdout.String())
	}
}

func TestHook_AmbiguousCdToRegisteredToolBlocks(t *testing.T) {
	// cd /tmp/x; kubectl get pods — kubectl is registered; ambiguous cwd.
	// Hook layer must refuse rather than evaluate against wrong cwd.
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"cd /tmp; kubectl get pods"},"cwd":"/tmp"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{Home: t.TempDir()})
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"decision":"block"`) {
		t.Errorf("expected block decision; got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ambiguous cd") {
		t.Errorf("expected reason to mention ambiguous cd; got: %s", stdout.String())
	}
}

func TestHook_AmbiguousCdToNonRegisteredToolAllows(t *testing.T) {
	// cd /tmp/x; echo hi — echo not registered → allow.
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"cd /tmp; echo hi"},"cwd":"/tmp"}`)
	var stdout, stderr bytes.Buffer
	code := Hook(in, &stdout, &stderr, HookOptions{Home: t.TempDir()})
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout (allow); got: %s", stdout.String())
	}
}
