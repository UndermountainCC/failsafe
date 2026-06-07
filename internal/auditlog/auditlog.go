// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package auditlog appends one JSON-Lines record per infrastructure-tool
// decision the hook makes (block / allow / allow_override), plus the
// refuse/parse blocks. It is observability, not enforcement: logging MUST
// NOT fail the hook (mirrors the enricher contract in design §3.6 —
// fail-with-partial, never fail-the-hook). The hook calls Log and ignores
// the returned error.
//
// Records are written through DefaultRedact so secrets in the command
// string (--token=, AWS_SECRET_ACCESS_KEY=, etc.) never reach the log.
package auditlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Record is one logged decision.
type Record struct {
	Time      time.Time
	Decision  string // "block" | "allow" | "allow_override"
	Reason    string // block reason or override reason; empty for a plain allow
	Mode      string // "enabled" | "disabled" (older log entries may carry legacy "read"/"read & write")
	Tool      string // registry tool name; empty for refuse/parse blocks
	Verb      string
	Subverb   string
	CWD       string // effective cwd for this call
	Command   string // raw command; redacted before write
	AgentType string // coding agent that triggered the hook, e.g. "claude-code"
	SessionID string // agent session id
	Pane      string // terminal pane id (WEZTERM_PANE etc.)
}

// ParseRecord decodes one JSON-Lines decision record back into a Record. It is
// the inverse of Log's marshaling and tolerates the legacy session schema via
// sessionJSON's migration shim (see UnmarshalJSON). The `report` subcommand uses
// it to read decisions.jsonl; malformed lines surface as an error so callers can
// skip-and-continue rather than abort the whole file.
func ParseRecord(line []byte) (Record, error) {
	var rj recordJSON
	if err := json.Unmarshal(line, &rj); err != nil {
		return Record{}, err
	}
	var ts time.Time
	if rj.Time != "" {
		parsed, err := time.Parse(time.RFC3339, rj.Time)
		if err != nil {
			return Record{}, err
		}
		ts = parsed
	}
	return Record{
		Time:      ts,
		Decision:  rj.Decision,
		Reason:    rj.Reason,
		Mode:      rj.Mode,
		Tool:      rj.Tool,
		Verb:      rj.Verb,
		Subverb:   rj.Subverb,
		CWD:       rj.CWD,
		Command:   rj.Command,
		AgentType: rj.Session.AgentType,
		SessionID: rj.Session.AgentSessionID,
		Pane:      rj.Session.TerminalPane,
	}, nil
}

// Logger appends Records as JSON lines. Exactly one sink is used: Writer
// (in-memory/tests) takes precedence over Path (a file appended to). With
// neither set, the logger is disabled and Log is a no-op.
type Logger struct {
	Writer interface {
		Write([]byte) (int, error)
	}
	Path   string
	Redact func(string) string // nil → DefaultRedact
}

// recordJSON is the on-disk shape. Field order/tags are the stable contract;
// omitempty keeps lines compact when fields don't apply (e.g. refuse blocks
// have no tool/verb).
type recordJSON struct {
	Time     string      `json:"ts"`
	Decision string      `json:"decision"`
	Reason   string      `json:"reason,omitempty"`
	Mode     string      `json:"mode,omitempty"`
	Tool     string      `json:"tool,omitempty"`
	Verb     string      `json:"verb,omitempty"`
	Subverb  string      `json:"subverb,omitempty"`
	CWD      string      `json:"cwd,omitempty"`
	Command  string      `json:"command,omitempty"`
	Session  sessionJSON `json:"session"`
}

// sessionJSON is the on-disk session block. The field tags are the post-
// clean-break schema (agent-neutral: failsafe now targets coding agents
// beyond Claude Code and terminals beyond WezTerm). UnmarshalJSON below keeps
// the reader backward-compatible with the pre-clean-break schema.
type sessionJSON struct {
	AgentType      string `json:"agent_type,omitempty"`
	AgentSessionID string `json:"agent_session_id"`
	TerminalPane   string `json:"terminal_pane,omitempty"`
}

// UnmarshalJSON is the migration shim. It decodes the new schema and falls back
// to the legacy keys (claude_session_id / wezterm_pane) only when the new field
// is absent — so an upgraded line never has its values clobbered by stale ones.
// Only the read path is affected: with no MarshalJSON defined, Log still writes
// the new schema via the struct tags above.
func (s *sessionJSON) UnmarshalJSON(b []byte) error {
	var raw struct {
		AgentType      string `json:"agent_type"`
		AgentSessionID string `json:"agent_session_id"`
		TerminalPane   string `json:"terminal_pane"`
		// legacy aliases (pre-clean-break schema)
		ClaudeSessionID string `json:"claude_session_id"`
		WeztermPane     string `json:"wezterm_pane"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.AgentType = raw.AgentType
	s.AgentSessionID = raw.AgentSessionID
	if s.AgentSessionID == "" {
		s.AgentSessionID = raw.ClaudeSessionID
	}
	s.TerminalPane = raw.TerminalPane
	if s.TerminalPane == "" {
		s.TerminalPane = raw.WeztermPane
	}
	return nil
}

// Log writes one JSON line. It never panics; on a disabled or nil logger it
// is a no-op. Errors (marshal, open, write) are returned for tests but the
// hook is expected to ignore them — logging must not block a command.
func (l *Logger) Log(r Record) error {
	if l == nil || (l.Writer == nil && l.Path == "") {
		return nil
	}
	redact := l.Redact
	if redact == nil {
		redact = DefaultRedact
	}
	line, err := json.Marshal(recordJSON{
		Time:     r.Time.UTC().Format(time.RFC3339),
		Decision: r.Decision,
		Reason:   r.Reason,
		Mode:     r.Mode,
		Tool:     r.Tool,
		Verb:     r.Verb,
		Subverb:  r.Subverb,
		CWD:      r.CWD,
		Command:  redact(r.Command),
		Session:  sessionJSON{AgentType: r.AgentType, AgentSessionID: r.SessionID, TerminalPane: r.Pane},
	})
	if err != nil {
		return err
	}
	line = append(line, '\n')

	if l.Writer != nil {
		_, err := l.Writer.Write(line)
		return err
	}

	// Owner-only (0o700 dir, 0o600 file): the log holds command lines, and a
	// redaction miss must not also become a cross-user disclosure. Redaction
	// is the primary control; these perms are the backstop.
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return err
	}
	// O_APPEND: a single Write of one line is atomic on POSIX for sizes under
	// PIPE_BUF, so concurrent panes interleave whole lines, not bytes.
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

// DefaultLogger resolves where decisions are logged:
//   - FAILSAFE_LOG="off"      → disabled
//   - FAILSAFE_LOG=<path>     → that path
//   - unset, home != ""         → $HOME/.config/failsafe/decisions.jsonl
//   - unset, home == ""         → disabled (no path computable)
//
// env is the environment accessor (os.Getenv in production, a stub in tests).
func DefaultLogger(home string, env func(string) string) *Logger {
	switch v := env("FAILSAFE_LOG"); {
	case v == "off":
		return &Logger{}
	case v != "":
		return &Logger{Path: v}
	case home != "":
		return &Logger{Path: filepath.Join(home, ".config", "failsafe", "decisions.jsonl")}
	default:
		return &Logger{}
	}
}

const redactMask = "***"

var (
	// --flag=VALUE where the flag name contains a secret-ish word.
	secretFlagEq = regexp.MustCompile(`(?i)(--?[a-z0-9_-]*(?:token|secret|password|passwd|credential|api[-_]?key|apikey|auth|bearer)[a-z0-9_-]*=)\S+`)
	// --flag VALUE (separated). The value must not itself start with '-' so a
	// following flag (--token --verbose) isn't mistaken for the secret value.
	secretFlagSp = regexp.MustCompile(`(?i)(--?[a-z0-9_-]*(?:token|secret|password|passwd|credential|api[-_]?key|apikey|auth|bearer)[a-z0-9_-]*\s+)[^\s-]\S*`)
	// KEY=VALUE env assignment with a secret-ish key (no dashes — that's a flag).
	secretEnv = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*(?:token|secret|password|passwd|credential|key|auth)[a-z0-9_]*=)\S+`)
)

// DefaultRedact masks secret-looking flag values and env assignments in a
// command string before it is logged. It errs toward over-redaction: a
// false positive hides a non-secret, which is harmless in an audit log; a
// false negative leaks a credential, which is not.
//
// This pattern set is deliberately conservative and easy to extend — tune it
// for your own infra's credential conventions.
func DefaultRedact(cmd string) string {
	cmd = secretFlagEq.ReplaceAllString(cmd, "${1}"+redactMask)
	cmd = secretFlagSp.ReplaceAllString(cmd, "${1}"+redactMask)
	cmd = secretEnv.ReplaceAllString(cmd, "${1}"+redactMask)
	return cmd
}
