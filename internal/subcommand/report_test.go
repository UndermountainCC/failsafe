// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedNow is the reference clock for every window-sensitive assertion.
var fixedNow = time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

func writeLog(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "decisions.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// reportEnvelope mirrors the --format json output. Declared in the test so the
// JSON contract is pinned independently of the production struct names.
type reportEnvelope struct {
	Since  string `json:"since"`
	Total  int    `json:"total"`
	Counts []struct {
		Tool     string `json:"tool"`
		Verb     string `json:"verb"`
		Decision string `json:"decision"`
		Count    int    `json:"count"`
	} `json:"counts"`
	Scariest []struct {
		Decision string `json:"decision"`
		Tool     string `json:"tool"`
		Verb     string `json:"verb"`
		Score    int    `json:"score"`
	} `json:"scariest"`
}

func runReportJSON(t *testing.T, logPath string, extraArgs ...string) reportEnvelope {
	t.Helper()
	var buf bytes.Buffer
	args := append([]string{"--format", "json"}, extraArgs...)
	code := Report(args, &buf, ReportOptions{Home: "/Users/you", LogPath: logPath, Now: fixedNow})
	if code != 0 {
		t.Fatalf("Report exit=%d, out=%s", code, buf.String())
	}
	var env reportEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("report json: %v\noutput: %s", err, buf.String())
	}
	return env
}

func TestParseSince(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"bogus", 0, true},
	}
	for _, tc := range cases {
		got, err := parseSince(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("parseSince(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSince(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("parseSince(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestReport_FiltersWindowAndSkipsMalformed(t *testing.T) {
	// Relative to fixedNow with default --since 7d, cutoff = 2026-05-23T12:00:00Z.
	log := writeLog(t,
		`{"ts":"2026-05-30T11:00:00Z","decision":"allow","tool":"kubectl","verb":"get","session":{"agent_session_id":"s"}}`,    // 1h ago — in
		`{"ts":"2026-05-23T12:00:00Z","decision":"block","tool":"kubectl","verb":"delete","session":{"agent_session_id":"s"}}`, // exactly at cutoff — in
		`{"ts":"2026-05-20T12:00:00Z","decision":"block","tool":"helm","verb":"uninstall","session":{"agent_session_id":"s"}}`, // 10d ago — out
		`{not valid json at all`, // malformed — skipped, must not abort
	)
	env := runReportJSON(t, log)
	if env.Total != 2 {
		t.Errorf("total = %d, want 2 (in-window, malformed skipped)", env.Total)
	}
}

func TestReport_MissingLogIsGraceful(t *testing.T) {
	var buf bytes.Buffer
	code := Report(nil, &buf, ReportOptions{Home: "/Users/you", LogPath: "/no/such/decisions.jsonl", Now: fixedNow})
	if code != 0 {
		t.Fatalf("missing log should exit 0, got %d", code)
	}
	if buf.Len() == 0 {
		t.Errorf("expected some output for an empty report, got nothing")
	}
}

func TestReport_AggregatesCounts(t *testing.T) {
	log := writeLog(t,
		`{"ts":"2026-05-30T11:00:00Z","decision":"block","tool":"kubectl","verb":"delete","session":{"agent_session_id":"s"}}`,
		`{"ts":"2026-05-30T11:01:00Z","decision":"block","tool":"kubectl","verb":"delete","session":{"agent_session_id":"s"}}`,
		`{"ts":"2026-05-30T11:02:00Z","decision":"allow","tool":"kubectl","verb":"get","session":{"agent_session_id":"s"}}`,
	)
	env := runReportJSON(t, log)
	var deleteBlocks int
	for _, c := range env.Counts {
		if c.Tool == "kubectl" && c.Verb == "delete" && c.Decision == "block" {
			deleteBlocks = c.Count
		}
	}
	if deleteBlocks != 2 {
		t.Errorf("kubectl/delete/block count = %d, want 2 (counts: %+v)", deleteBlocks, env.Counts)
	}
}

func TestReport_ScariestOrdering(t *testing.T) {
	log := writeLog(t,
		`{"ts":"2026-05-30T11:00:00Z","decision":"allow","tool":"kubectl","verb":"get","session":{"agent_session_id":"s"}}`,                                                                    // not scary
		`{"ts":"2026-05-30T11:01:00Z","decision":"allow_override","tool":"terraform","verb":"apply","reason":"override","session":{"agent_session_id":"s"}}`,                                   // mid
		`{"ts":"2026-05-30T11:02:00Z","decision":"block","tool":"kubectl","verb":"apply","reason":"blocked in read mode","session":{"agent_session_id":"s"}}`,                                  // high
		`{"ts":"2026-05-30T11:03:00Z","decision":"block","tool":"kubectl","verb":"delete","cwd":"/Users/you/Code/prod","reason":"blocked in read mode","session":{"agent_session_id":"s"}}`, // highest (prod)
	)
	env := runReportJSON(t, log)
	if len(env.Scariest) < 3 {
		t.Fatalf("expected at least 3 scariest entries, got %d: %+v", len(env.Scariest), env.Scariest)
	}
	// A plain allow is never scary.
	for _, s := range env.Scariest {
		if s.Decision == "allow" {
			t.Errorf("plain allow should not appear in scariest: %+v", s)
		}
	}
	// Descending by score: block+prod, then block, then override.
	if !(env.Scariest[0].Score >= env.Scariest[1].Score && env.Scariest[1].Score >= env.Scariest[2].Score) {
		t.Errorf("scariest not sorted by score desc: %+v", env.Scariest)
	}
	top := env.Scariest[0]
	if top.Decision != "block" || top.Verb != "delete" {
		t.Errorf("top scariest should be the blocked prod delete; got %+v", top)
	}
}

func TestReport_DefaultFormatIsMarkdown(t *testing.T) {
	log := writeLog(t,
		`{"ts":"2026-05-30T11:00:00Z","decision":"block","tool":"kubectl","verb":"delete","session":{"agent_session_id":"s"}}`,
	)
	var buf bytes.Buffer
	code := Report(nil, &buf, ReportOptions{Home: "/Users/you", LogPath: log, Now: fixedNow})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "#") {
		t.Errorf("default output should be markdown (expected a heading); got:\n%s", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("default output should be markdown, not JSON; got:\n%s", out)
	}
}

// TestReport_ShareRedactsOutput is the §2/§3 join: a --share run must never emit
// raw $HOME paths, usernames, ARNs, or account ids anywhere in the rendered
// report.
func TestReport_ShareRedactsOutput(t *testing.T) {
	log := writeLog(t,
		`{"ts":"2026-05-30T11:00:00Z","decision":"block","tool":"kubectl","verb":"delete","cwd":"/Users/you/Code/infra","reason":"would touch /Users/you via arn:aws:eks:us-east-1:123456789012:cluster/prod","command":"kubectl --context arn:aws:eks:us-east-1:123456789012:cluster/prod delete pod x","session":{"agent_session_id":"s"}}`,
	)
	var buf bytes.Buffer
	code := Report([]string{"--share"}, &buf, ReportOptions{Home: "/Users/you", LogPath: log, Now: fixedNow})
	if code != 0 {
		t.Fatalf("exit=%d, out=%s", code, buf.String())
	}
	out := buf.String()
	for _, raw := range []string{"/Users/you", "you", "123456789012", "arn:aws:eks:us-east-1"} {
		if strings.Contains(out, raw) {
			t.Errorf("--share report leaked %q:\n%s", raw, out)
		}
	}
}

// Without --share, the report is local-only and keeps raw values (they're useful
// locally and the file never leaves the machine).
func TestReport_NoShareKeepsRawValues(t *testing.T) {
	log := writeLog(t,
		`{"ts":"2026-05-30T11:00:00Z","decision":"block","tool":"kubectl","verb":"delete","cwd":"/Users/you/Code/infra","command":"kubectl delete pod x","session":{"agent_session_id":"s"}}`,
	)
	var buf bytes.Buffer
	if code := Report(nil, &buf, ReportOptions{Home: "/Users/you", LogPath: log, Now: fixedNow}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(buf.String(), "/Users/you/Code/infra") {
		t.Errorf("local report should keep raw cwd; got:\n%s", buf.String())
	}
}

// TestReport_SubverbColumnBreaksDownRefusalKinds verifies that shell-parser refusals
// with different subverbs (e.g. glob vs heredoc) produce distinct rows in the report
// and that a normal tool row (git/commit) still renders correctly with an empty Subverb.
func TestReport_SubverbColumnBreaksDownRefusalKinds(t *testing.T) {
	log := writeLog(t,
		// Two distinct shell/unanalyzable refusals: glob and heredoc
		`{"ts":"2026-05-30T11:00:00Z","decision":"block","tool":"shell","verb":"unanalyzable","subverb":"glob","session":{"agent_session_id":"s"}}`,
		`{"ts":"2026-05-30T11:01:00Z","decision":"block","tool":"shell","verb":"unanalyzable","subverb":"heredoc","session":{"agent_session_id":"s"}}`,
		// A normal tool row with no subverb
		`{"ts":"2026-05-30T11:02:00Z","decision":"allow","tool":"git","verb":"commit","session":{"agent_session_id":"s"}}`,
	)
	env := runReportJSON(t, log)
	if env.Total != 3 {
		t.Fatalf("total = %d, want 3", env.Total)
	}

	// Build a map of (tool,verb,decision) → rows to make lookup easy.
	type rowKey struct{ tool, verb, decision string }
	// We need to capture subverb too; use a local struct for richer lookup.
	type fullRow struct {
		Tool, Verb, Subverb, Decision string
		Count                         int
	}
	// Re-run with JSON to get subverb column.
	var rawBuf bytes.Buffer
	args := []string{"--format", "json"}
	code := Report(args, &rawBuf, ReportOptions{Home: "/Users/you", LogPath: log, Now: fixedNow})
	if code != 0 {
		t.Fatalf("Report exit=%d, out=%s", code, rawBuf.String())
	}
	// Decode with a richer struct that includes Subverb.
	var richEnv struct {
		Counts []struct {
			Tool     string `json:"tool"`
			Verb     string `json:"verb"`
			Subverb  string `json:"subverb"`
			Decision string `json:"decision"`
			Count    int    `json:"count"`
		} `json:"counts"`
	}
	if err := json.Unmarshal(rawBuf.Bytes(), &richEnv); err != nil {
		t.Fatalf("json decode: %v\noutput: %s", err, rawBuf.String())
	}

	// We expect three distinct rows (glob block, heredoc block, git/commit allow).
	if len(richEnv.Counts) != 3 {
		t.Fatalf("expected 3 distinct count rows (glob+heredoc refusals + git/commit), got %d: %+v", len(richEnv.Counts), richEnv.Counts)
	}

	var sawGlob, sawHeredoc, sawGitCommit bool
	for _, c := range richEnv.Counts {
		switch {
		case c.Tool == "shell" && c.Verb == "unanalyzable" && c.Subverb == "glob" && c.Decision == "block":
			sawGlob = true
			if c.Count != 1 {
				t.Errorf("glob block count = %d, want 1", c.Count)
			}
		case c.Tool == "shell" && c.Verb == "unanalyzable" && c.Subverb == "heredoc" && c.Decision == "block":
			sawHeredoc = true
			if c.Count != 1 {
				t.Errorf("heredoc block count = %d, want 1", c.Count)
			}
		case c.Tool == "git" && c.Verb == "commit" && c.Decision == "allow":
			sawGitCommit = true
			if c.Subverb != "" {
				t.Errorf("git/commit subverb = %q, want empty", c.Subverb)
			}
		}
	}
	if !sawGlob {
		t.Errorf("missing shell/unanalyzable/glob/block row; counts: %+v", richEnv.Counts)
	}
	if !sawHeredoc {
		t.Errorf("missing shell/unanalyzable/heredoc/block row; counts: %+v", richEnv.Counts)
	}
	if !sawGitCommit {
		t.Errorf("missing git/commit/allow row; counts: %+v", richEnv.Counts)
	}
}

// TestReport_MarkdownSubverbColumn checks that the markdown table header and rows
// include the Subverb column.
func TestReport_MarkdownSubverbColumn(t *testing.T) {
	log := writeLog(t,
		`{"ts":"2026-05-30T11:00:00Z","decision":"block","tool":"shell","verb":"unanalyzable","subverb":"glob","session":{"agent_session_id":"s"}}`,
		`{"ts":"2026-05-30T11:01:00Z","decision":"allow","tool":"git","verb":"commit","session":{"agent_session_id":"s"}}`,
	)
	var buf bytes.Buffer
	if code := Report(nil, &buf, ReportOptions{Home: "/Users/you", LogPath: log, Now: fixedNow}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "Subverb") {
		t.Errorf("markdown table should contain Subverb column header; got:\n%s", out)
	}
	if !strings.Contains(out, "glob") {
		t.Errorf("markdown table should contain 'glob' subverb value; got:\n%s", out)
	}
}
