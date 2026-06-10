// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/UndermountainCC/failsafe/internal/auditlog"
)

// ReportOptions configures the report subcommand. Now and LogPath are injected
// so the window logic and log source are deterministic under test.
type ReportOptions struct {
	Home    string
	LogPath string    // decisions.jsonl; empty → resolve from Home
	Now     time.Time // reference clock for --since; zero → time.Now()
}

// Report reads the decision log, filters to a time window, aggregates by
// tool+verb+decision, surfaces the top-N "scariest" decisions, and renders the
// result. It is local-only: deployment-identifying data stays raw unless --share
// is passed, which applies auditlog.RedactForShare to every record first.
//
// Missing or empty logs are not errors — a fresh install simply has nothing to
// report yet.
func Report(args []string, out io.Writer, opts ReportOptions) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(out)
	since := fs.String("since", "7d", "time window, e.g. 7d, 24h, 30m")
	format := fs.String("format", "md", "output format: md | json")
	share := fs.Bool("share", false, "redact deployment-identifying data for safe sharing")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	window, err := parseSince(*since)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	if *format != "md" && *format != "json" {
		fmt.Fprintf(out, "unknown --format %q (want md or json)\n", *format)
		return 2
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-window)

	path := opts.LogPath
	if path == "" {
		path = defaultLogPath(opts.Home)
	}

	recs, err := readRecords(path, cutoff)
	if err != nil {
		fmt.Fprintf(out, "report: %v\n", err)
		return 1
	}
	if *share {
		for i := range recs {
			recs[i] = auditlog.RedactForShare(opts.Home, recs[i])
		}
	}

	rep := buildReport(*since, recs)
	if *format == "json" {
		return renderJSON(out, rep)
	}
	return renderMarkdown(out, rep)
}

func defaultLogPath(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "failsafe", "decisions.jsonl")
}

// parseSince accepts a bare day count (e.g. "7d") or any Go duration string
// (e.g. "24h", "30m"). Days are spelled with a trailing 'd' because Go's
// time.ParseDuration has no day unit.
func parseSince(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid --since %q: %v", s, err)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: %v", s, err)
	}
	return d, nil
}

// readRecords reads decisions.jsonl, keeping records at or after cutoff. A
// missing file yields no records (not an error). Malformed lines are skipped so
// one corrupt write never hides the rest of the trail.
func readRecords(path string, cutoff time.Time) ([]auditlog.Record, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var recs []auditlog.Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // command lines can be long
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		rec, err := auditlog.ParseRecord(line)
		if err != nil {
			continue // skip malformed line
		}
		if rec.Time.Before(cutoff) {
			continue // outside window (cutoff itself is included)
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return recs, nil
}

// scariness scores how alarming a decision is. A plain allow is never scary; a
// block (the guard stopped a mutation) outranks an allow_override (a guard was
// bypassed), and prod-ish context bumps either. The heuristic is intentionally
// small and tunable — adjust the weights for your own infra's risk model.
func scariness(r auditlog.Record) int {
	score := 0
	switch r.Decision {
	case "block":
		score += 10
	case "allow_override":
		score += 5
	}
	if score == 0 {
		return 0
	}
	hay := strings.ToLower(r.CWD + " " + r.Reason + " " + r.Command)
	if strings.Contains(hay, "prod") {
		score += 3
	}
	return score
}

type countRow struct {
	Tool     string `json:"tool"`
	Verb     string `json:"verb"`
	Subverb  string `json:"subverb,omitempty"`
	Decision string `json:"decision"`
	Count    int    `json:"count"`
}

type scariestRow struct {
	Time     string `json:"ts"`
	Decision string `json:"decision"`
	Tool     string `json:"tool"`
	Verb     string `json:"verb"`
	CWD      string `json:"cwd,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Command  string `json:"command,omitempty"`
	Score    int    `json:"score"`
}

type report struct {
	Since    string        `json:"since"`
	Total    int           `json:"total"`
	Counts   []countRow    `json:"counts"`
	Scariest []scariestRow `json:"scariest"`
}

const scariestN = 5

func buildReport(since string, recs []auditlog.Record) report {
	// Aggregate by tool+verb+subverb+decision.
	type key struct{ tool, verb, subverb, decision string }
	tally := map[key]int{}
	for _, r := range recs {
		tally[key{r.Tool, r.Verb, r.Subverb, r.Decision}]++
	}
	counts := make([]countRow, 0, len(tally))
	for k, n := range tally {
		counts = append(counts, countRow{Tool: k.tool, Verb: k.verb, Subverb: k.subverb, Decision: k.decision, Count: n})
	}
	// Deterministic order: most frequent first, then alphabetical for stable ties.
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].Count != counts[j].Count {
			return counts[i].Count > counts[j].Count
		}
		if counts[i].Tool != counts[j].Tool {
			return counts[i].Tool < counts[j].Tool
		}
		if counts[i].Verb != counts[j].Verb {
			return counts[i].Verb < counts[j].Verb
		}
		if counts[i].Subverb != counts[j].Subverb {
			return counts[i].Subverb < counts[j].Subverb
		}
		return counts[i].Decision < counts[j].Decision
	})

	// Top-N scariest: score > 0 only, by score desc then most recent first.
	scary := make([]scariestRow, 0)
	for _, r := range recs {
		s := scariness(r)
		if s == 0 {
			continue
		}
		scary = append(scary, scariestRow{
			Time: r.Time.UTC().Format(time.RFC3339), Decision: r.Decision,
			Tool: r.Tool, Verb: r.Verb, CWD: r.CWD, Reason: r.Reason,
			Command: r.Command, Score: s,
		})
	}
	sort.SliceStable(scary, func(i, j int) bool {
		if scary[i].Score != scary[j].Score {
			return scary[i].Score > scary[j].Score
		}
		return scary[i].Time > scary[j].Time // RFC3339 sorts lexically by time
	})
	if len(scary) > scariestN {
		scary = scary[:scariestN]
	}

	return report{Since: since, Total: len(recs), Counts: counts, Scariest: scary}
}

func renderJSON(out io.Writer, rep report) int {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		fmt.Fprintf(out, "report: %v\n", err)
		return 1
	}
	return 0
}

func renderMarkdown(out io.Writer, rep report) int {
	fmt.Fprintf(out, "# failsafe decision report\n\n")
	fmt.Fprintf(out, "Window: last %s — %d decision(s).\n\n", rep.Since, rep.Total)
	if rep.Total == 0 {
		fmt.Fprintln(out, "No decisions logged in this window.")
		return 0
	}

	fmt.Fprintf(out, "## Counts by tool / verb / decision\n\n")
	fmt.Fprintln(out, "| Tool | Verb | Subverb | Decision | Count |")
	fmt.Fprintln(out, "|------|------|---------|----------|-------|")
	for _, c := range rep.Counts {
		fmt.Fprintf(out, "| %s | %s | %s | %s | %d |\n", c.Tool, c.Verb, c.Subverb, c.Decision, c.Count)
	}

	if len(rep.Scariest) > 0 {
		fmt.Fprintf(out, "\n## Scariest decisions\n\n")
		for _, s := range rep.Scariest {
			fmt.Fprintf(out, "- **%s** %s %s (score %d)%s — %s\n", s.Decision, s.Tool, s.Verb, s.Score, scariestWhere(s), scariestDetail(s))
		}
	}
	return 0
}

// scariestWhere renders the effective cwd of a scary decision — knowing WHERE a
// dangerous command was run is most of the context. Empty when unknown.
func scariestWhere(s scariestRow) string {
	if s.CWD == "" {
		return ""
	}
	return " in `" + s.CWD + "`"
}

func scariestDetail(s scariestRow) string {
	if s.Reason != "" {
		return s.Reason
	}
	if s.Command != "" {
		return "`" + s.Command + "`"
	}
	return "(no detail)"
}
