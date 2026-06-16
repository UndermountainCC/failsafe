// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// logFixture is a small JSONL fixture covering the main cases: a block with
// tool/verb/subverb + command + reason, a plain allow, and a refuse block.
var logFixture = []string{
	// block shell/unanalyzable/glob with command and reason
	`{"ts":"2026-05-30T11:00:00Z","decision":"block","tool":"shell","verb":"unanalyzable","subverb":"glob","cwd":"/repo","reason":"cannot safely analyze: unquoted glob","command":"rm -rf build/*.tmp","session":{"agent_session_id":"s1"}}`,
	// plain allow (kubectl get)
	`{"ts":"2026-05-30T11:01:00Z","decision":"allow","tool":"kubectl","verb":"get","cwd":"/repo","session":{"agent_session_id":"s1"}}`,
	// allow_override with reason and command
	`{"ts":"2026-05-30T11:02:00Z","decision":"allow_override","tool":"terraform","verb":"apply","cwd":"/infra","reason":"user-approved","command":"terraform apply -auto-approve","session":{"agent_session_id":"s1"}}`,
	// block without command (reason only)
	`{"ts":"2026-05-30T11:03:00Z","decision":"block","tool":"kubectl","verb":"delete","cwd":"/infra","reason":"blocked in read mode","session":{"agent_session_id":"s1"}}`,
}

func TestLog_FormattedOutput(t *testing.T) {
	path := writeLog(t, logFixture...)
	var buf bytes.Buffer
	code := Log(nil, &buf, LogOptions{Home: "/Users/you", LogPath: path, Now: fixedNow})
	if code != 0 {
		t.Fatalf("Log exit=%d, out=%s", code, buf.String())
	}
	out := buf.String()

	// The shell block should show category, reason, and command.
	if !strings.Contains(out, "shell/unanalyzable/glob") {
		t.Errorf("output should contain category 'shell/unanalyzable/glob'; got:\n%s", out)
	}
	if !strings.Contains(out, "cannot safely analyze: unquoted glob") {
		t.Errorf("output should contain the reason; got:\n%s", out)
	}
	if !strings.Contains(out, "rm -rf build/*.tmp") {
		t.Errorf("output should contain the command; got:\n%s", out)
	}

	// All four records should appear (no --tail reduction since 4 < default 20).
	for _, want := range []string{"block", "allow", "allow_override"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain decision %q; got:\n%s", want, out)
		}
	}
}

func TestLog_TailLimitsOutput(t *testing.T) {
	path := writeLog(t, logFixture...)
	var buf bytes.Buffer
	code := Log([]string{"--tail", "2"}, &buf, LogOptions{Home: "/Users/you", LogPath: path, Now: fixedNow})
	if code != 0 {
		t.Fatalf("Log exit=%d, out=%s", code, buf.String())
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("--tail 2 should produce 2 lines, got %d:\n%s", len(lines), out)
	}
	// Should be the last 2 records: allow_override and the no-command block.
	if !strings.Contains(out, "allow_override") {
		t.Errorf("--tail 2 should include the allow_override record; got:\n%s", out)
	}
}

func TestLog_SinceFiltersWindow(t *testing.T) {
	// fixedNow = 2026-05-30T12:00:00Z; --since 1h → cutoff 2026-05-30T11:00:00Z.
	// All four fixture records are at 11:00–11:03, so all should be in-window.
	path := writeLog(t, logFixture...)
	var buf bytes.Buffer
	code := Log([]string{"--since", "1h"}, &buf, LogOptions{Home: "/Users/you", LogPath: path, Now: fixedNow})
	if code != 0 {
		t.Fatalf("Log exit=%d, out=%s", code, buf.String())
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Errorf("--since 1h should include all 4 records, got %d", len(lines))
	}
}

func TestLog_SinceExcludesOldRecords(t *testing.T) {
	// --since 30m from fixedNow means cutoff = 2026-05-30T11:30:00Z.
	// All fixture records (11:00–11:03) are before the cutoff → none in window.
	path := writeLog(t, logFixture...)
	var buf bytes.Buffer
	code := Log([]string{"--since", "30m"}, &buf, LogOptions{Home: "/Users/you", LogPath: path, Now: fixedNow})
	if code != 0 {
		t.Fatalf("Log exit=%d, out=%s", code, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "No decisions logged yet.") {
		t.Errorf("--since 30m should produce empty result message; got:\n%s", out)
	}
}

func TestLog_JSONOutput(t *testing.T) {
	path := writeLog(t, logFixture...)
	var buf bytes.Buffer
	code := Log([]string{"--json"}, &buf, LogOptions{Home: "/Users/you", LogPath: path, Now: fixedNow})
	if code != 0 {
		t.Fatalf("Log exit=%d, out=%s", code, buf.String())
	}
	out := buf.String()

	// Each line should be valid JSON.
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("--json line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
	}

	// The shell/glob block record should appear with its fields.
	if !strings.Contains(out, "shell") {
		t.Errorf("--json output should contain 'shell'; got:\n%s", out)
	}
	if !strings.Contains(out, "rm -rf build") {
		t.Errorf("--json output should contain the command; got:\n%s", out)
	}
}

func TestLog_MissingLogIsFriendly(t *testing.T) {
	var buf bytes.Buffer
	code := Log(nil, &buf, LogOptions{Home: "/Users/you", LogPath: "/no/such/file.jsonl", Now: fixedNow})
	if code != 0 {
		t.Fatalf("missing log should exit 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "No decisions logged yet.") {
		t.Errorf("missing log should print friendly message; got:\n%s", out)
	}
}

func TestLog_EmptyFileIsFriendly(t *testing.T) {
	// writeLog with no records still writes a single newline; test empty-ish file.
	path := writeLog(t) // no lines → produces just "\n"
	var buf bytes.Buffer
	code := Log(nil, &buf, LogOptions{Home: "/Users/you", LogPath: path, Now: fixedNow})
	if code != 0 {
		t.Fatalf("empty log should exit 0, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "No decisions logged yet.") {
		t.Errorf("empty log should print friendly message; got:\n%s", out)
	}
}

func TestLog_InvalidSince(t *testing.T) {
	path := writeLog(t, logFixture...)
	var buf bytes.Buffer
	code := Log([]string{"--since", "bogus"}, &buf, LogOptions{Home: "/Users/you", LogPath: path, Now: fixedNow})
	if code != 2 {
		t.Errorf("invalid --since should exit 2, got %d", code)
	}
}
