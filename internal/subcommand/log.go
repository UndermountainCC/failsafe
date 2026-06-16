// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/UndermountainCC/failsafe/internal/auditlog"
)

// LogOptions configures the log subcommand. LogPath is injected so the log
// source is deterministic under test; Home is used only when LogPath is empty.
type LogOptions struct {
	Home    string
	LogPath string    // decisions.jsonl path; empty → resolve from Home
	Now     time.Time // reference clock for --since; zero → time.Now()
}

// Log reads the decision log and prints recent decisions. It is the inspection
// companion to `report`: where report aggregates and scores, log shows raw
// entries so an operator can see exactly what was blocked and why.
//
// Flags:
//   - --tail N (default 20): print the last N records.
//   - --since DUR (e.g. 7d, 24h): filter to records within this window.
//   - --json: emit raw JSON-Lines records instead of formatted output.
//
// Missing or empty log files are not errors — a fresh install has nothing to
// show yet.
func Log(args []string, out io.Writer, opts LogOptions) int {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	fs.SetOutput(out)
	tail := fs.Int("tail", 20, "print the last N records")
	since := fs.String("since", "", "time window, e.g. 7d, 24h, 30m (default: no window filter)")
	jsonOut := fs.Bool("json", false, "emit raw JSON-Lines records")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	// Determine cutoff from --since, if given.
	var cutoff time.Time
	if *since != "" {
		window, err := parseSince(*since)
		if err != nil {
			fmt.Fprintln(out, err)
			return 2
		}
		cutoff = now.Add(-window)
	}

	path := opts.LogPath
	if path == "" && opts.Home != "" {
		path = filepath.Join(opts.Home, ".config", "failsafe", "decisions.jsonl")
	}

	// readRecords already handles missing files gracefully (returns nil, nil).
	recs, err := readRecords(path, cutoff)
	if err != nil {
		fmt.Fprintf(out, "log: %v\n", err)
		return 1
	}

	if len(recs) == 0 {
		fmt.Fprintln(out, "No decisions logged yet.")
		return 0
	}

	// Apply --tail: keep the last N records.
	if *tail > 0 && len(recs) > *tail {
		recs = recs[len(recs)-*tail:]
	}

	if *jsonOut {
		return renderLogJSON(out, recs)
	}
	return renderLogText(out, recs)
}

// renderLogJSON emits each record as a JSON line (raw re-marshal from the
// parsed Record). This lets operators pipe to jq, grep, etc.
func renderLogJSON(out io.Writer, recs []auditlog.Record) int {
	enc := json.NewEncoder(out)
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(out, "log: %v\n", err)
			return 1
		}
	}
	return 0
}

// renderLogText prints a human-readable list of decisions — one per line —
// with time, decision, category, cwd, reason, and command.
func renderLogText(out io.Writer, recs []auditlog.Record) int {
	for _, r := range recs {
		cat := logCategory(r)
		ts := r.Time.UTC().Format("2006-01-02T15:04:05Z")
		line := fmt.Sprintf("%s  %-14s  %s", ts, r.Decision, cat)
		if r.CWD != "" {
			line += "  [" + r.CWD + "]"
		}
		if r.Reason != "" {
			line += "  " + r.Reason
		}
		if r.Command != "" {
			line += "  `" + r.Command + "`"
		}
		fmt.Fprintln(out, line)
	}
	return 0
}

// logCategory builds the tool/verb/subverb category string for a record.
func logCategory(r auditlog.Record) string {
	if r.Tool == "" {
		return ""
	}
	cat := r.Tool
	if r.Verb != "" {
		cat += "/" + r.Verb
	}
	if r.Subverb != "" {
		cat += "/" + r.Subverb
	}
	return cat
}
