// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestLogger_WritesJSONLineToWriter(t *testing.T) {
	var buf bytes.Buffer
	lg := &Logger{Writer: &buf}
	err := lg.Log(Record{
		Time:      mustTime(t, "2026-05-28T12:00:00Z"),
		Decision:  "block",
		Reason:    "kubectl apply blocked in read mode",
		Mode:      "read",
		Tool:      "kubectl",
		Verb:      "apply",
		CWD:       "/Users/you/Code/infra",
		Command:   "kubectl apply -f prod.yaml",
		AgentType: "claude-code",
		SessionID: "sess-abc",
		Pane:      "42",
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("line must end in newline; got %q", out)
	}
	for _, want := range []string{
		`"ts":"2026-05-28T12:00:00Z"`,
		`"decision":"block"`,
		`"reason":"kubectl apply blocked in read mode"`,
		`"mode":"read"`,
		`"tool":"kubectl"`,
		`"verb":"apply"`,
		`"cwd":"/Users/you/Code/infra"`,
		`"command":"kubectl apply -f prod.yaml"`,
		`"agent_type":"claude-code"`,
		`"agent_session_id":"sess-abc"`,
		`"terminal_pane":"42"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %s\ngot: %s", want, out)
		}
	}
}

// TestParseRecord_LegacySchemaStillParses is the CRITICAL migration regression
// test. A decisions.jsonl line written by the pre-clean-break schema
// (session.claude_session_id / session.wezterm_pane) MUST still parse after the
// rename to agent_session_id / terminal_pane — otherwise existing log files are
// orphaned the moment we ship. The shim maps the legacy keys onto
// the new Record fields.
func TestParseRecord_LegacySchemaStillParses(t *testing.T) {
	legacy := `{"ts":"2026-05-28T12:00:00Z","decision":"block","reason":"kubectl apply blocked in read mode","mode":"read","tool":"kubectl","verb":"apply","cwd":"/Users/you/Code/infra","command":"kubectl apply -f prod.yaml","session":{"claude_session_id":"sess-abc","wezterm_pane":"42"}}`
	rec, err := ParseRecord([]byte(legacy))
	if err != nil {
		t.Fatalf("ParseRecord(legacy): %v", err)
	}
	if rec.SessionID != "sess-abc" {
		t.Errorf("legacy claude_session_id must map to SessionID; got %q", rec.SessionID)
	}
	if rec.Pane != "42" {
		t.Errorf("legacy wezterm_pane must map to Pane; got %q", rec.Pane)
	}
	if !rec.Time.Equal(mustTime(t, "2026-05-28T12:00:00Z")) {
		t.Errorf("ts mis-parsed; got %v", rec.Time)
	}
	if rec.Decision != "block" || rec.Mode != "read" || rec.Tool != "kubectl" || rec.Verb != "apply" {
		t.Errorf("common fields mis-parsed: %+v", rec)
	}
	if rec.CWD != "/Users/you/Code/infra" || rec.Command != "kubectl apply -f prod.yaml" {
		t.Errorf("cwd/command mis-parsed: cwd=%q command=%q", rec.CWD, rec.Command)
	}
}

// TestParseRecord_NewSchema pins the post-clean-break shape: agent_type +
// agent_session_id + terminal_pane decode onto AgentType / SessionID / Pane.
func TestParseRecord_NewSchema(t *testing.T) {
	line := `{"ts":"2026-05-28T12:00:00Z","decision":"allow","tool":"kubectl","verb":"get","session":{"agent_type":"claude-code","agent_session_id":"sess-xyz","terminal_pane":"7"}}`
	rec, err := ParseRecord([]byte(line))
	if err != nil {
		t.Fatalf("ParseRecord(new): %v", err)
	}
	if rec.AgentType != "claude-code" {
		t.Errorf("agent_type must map to AgentType; got %q", rec.AgentType)
	}
	if rec.SessionID != "sess-xyz" {
		t.Errorf("agent_session_id must map to SessionID; got %q", rec.SessionID)
	}
	if rec.Pane != "7" {
		t.Errorf("terminal_pane must map to Pane; got %q", rec.Pane)
	}
}

func TestLogger_AppendsOneLinePerRecord(t *testing.T) {
	var buf bytes.Buffer
	lg := &Logger{Writer: &buf}
	_ = lg.Log(Record{Time: mustTime(t, "2026-05-28T12:00:00Z"), Decision: "allow", Tool: "kubectl", Verb: "get"})
	_ = lg.Log(Record{Time: mustTime(t, "2026-05-28T12:00:01Z"), Decision: "block", Tool: "kubectl", Verb: "delete"})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], `"verb":"get"`) || !strings.Contains(lines[1], `"verb":"delete"`) {
		t.Errorf("lines out of order or wrong: %q", buf.String())
	}
}

func TestLogger_DisabledIsNoop(t *testing.T) {
	lg := &Logger{} // no Writer, no Path
	if err := lg.Log(Record{Decision: "block"}); err != nil {
		t.Errorf("disabled logger should no-op, got err %v", err)
	}
}

func TestLogger_NilReceiverIsNoop(t *testing.T) {
	var lg *Logger
	if err := lg.Log(Record{Decision: "block"}); err != nil {
		t.Errorf("nil logger should no-op, got err %v", err)
	}
}

func TestLogger_WritesToFileAppendingAndCreatingDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "decisions.jsonl")
	lg := &Logger{Path: path}
	if err := lg.Log(Record{Time: mustTime(t, "2026-05-28T12:00:00Z"), Decision: "allow", Tool: "git", Verb: "push"}); err != nil {
		t.Fatalf("Log 1: %v", err)
	}
	if err := lg.Log(Record{Time: mustTime(t, "2026-05-28T12:00:01Z"), Decision: "block", Tool: "kubectl", Verb: "delete"}); err != nil {
		t.Fatalf("Log 2: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 appended lines, got %d: %q", len(lines), string(body))
	}
}

func TestLogger_FileAndDirAreOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "failsafe", "decisions.jsonl")
	lg := &Logger{Path: path}
	if err := lg.Log(Record{Time: mustTime(t, "2026-05-28T12:00:00Z"), Decision: "block", Command: "kubectl --token=abc get"}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("log file perm = %o, want 600 (a decision log may hold un-redacted leftovers)", perm)
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("log dir perm = %o, want 700", perm)
	}
}

func TestLogger_RedactsCommandBeforeWriting(t *testing.T) {
	var buf bytes.Buffer
	lg := &Logger{Writer: &buf}
	_ = lg.Log(Record{
		Time:     mustTime(t, "2026-05-28T12:00:00Z"),
		Decision: "allow",
		Tool:     "helm",
		Verb:     "install",
		Command:  "AWS_SECRET_ACCESS_KEY=abcd1234 helm install --password hunter2 app",
	})
	out := buf.String()
	if strings.Contains(out, "abcd1234") || strings.Contains(out, "hunter2") {
		t.Errorf("secrets leaked into log: %s", out)
	}
	if !strings.Contains(out, "AWS_SECRET_ACCESS_KEY=***") {
		t.Errorf("expected env secret redacted; got: %s", out)
	}
	if !strings.Contains(out, "--password ***") {
		t.Errorf("expected flag secret redacted; got: %s", out)
	}
}

func TestDefaultRedact(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"token equals", "kubectl --token=SECRETVAL get", "kubectl --token=*** get"},
		{"password space", "helm install --password hunter2", "helm install --password ***"},
		{"env secret key", "AWS_SECRET_ACCESS_KEY=abc kubectl get", "AWS_SECRET_ACCESS_KEY=*** kubectl get"},
		{"bearer flag", "curl --bearer ABC123 https://x", "curl --bearer *** https://x"},
		{"benign context unchanged", "kubectl --context arn:aws:eks:cluster/prod get pods", "kubectl --context arn:aws:eks:cluster/prod get pods"},
		{"benign git unchanged", "git push origin main", "git push origin main"},
		{"value-is-flag not eaten", "kubectl --token --verbose", "kubectl --token --verbose"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DefaultRedact(tc.in); got != tc.want {
				t.Errorf("DefaultRedact(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDefaultLogger_PathResolution(t *testing.T) {
	home := "/home/user"
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	// Default: under home config dir.
	lg := DefaultLogger(home, env(map[string]string{}))
	wantPath := filepath.Join(home, ".config", "failsafe", "decisions.jsonl")
	if lg.Path != wantPath {
		t.Errorf("default path = %q, want %q", lg.Path, wantPath)
	}

	// Env override to explicit path.
	lg = DefaultLogger(home, env(map[string]string{"FAILSAFE_LOG": "/tmp/custom.jsonl"}))
	if lg.Path != "/tmp/custom.jsonl" {
		t.Errorf("env override path = %q, want /tmp/custom.jsonl", lg.Path)
	}

	// Disabled via "off".
	lg = DefaultLogger(home, env(map[string]string{"FAILSAFE_LOG": "off"}))
	if lg.Path != "" || lg.Writer != nil {
		t.Errorf("expected disabled logger for FAILSAFE_LOG=off, got Path=%q", lg.Path)
	}

	// No home, no env → disabled (can't compute a path).
	lg = DefaultLogger("", env(map[string]string{}))
	if lg.Path != "" {
		t.Errorf("expected disabled logger with empty home, got Path=%q", lg.Path)
	}
}
