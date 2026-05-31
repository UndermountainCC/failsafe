// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package hookio handles Claude Code PreToolUse hook JSON: parse stdin into
// a typed Input, and emit either an allow (exit 0, no stdout) or a block
// (stdout JSON of the decision-block shape Claude Code expects).
package hookio

import (
	"encoding/json"
	"io"
)

// Input is the relevant subset of Claude Code's PreToolUse hook JSON.
// Other fields exist (transcript_path, hook_event_name, etc.) but the engine
// doesn't use them today; add fields as needed.
type Input struct {
	ToolName  string    `json:"tool_name"`
	ToolInput ToolInput `json:"tool_input"`
	CWD       string    `json:"cwd"`
	SessionID string    `json:"session_id"`
}

type ToolInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// Read parses Claude Code hook JSON from r.
func Read(r io.Reader) (Input, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return Input{}, err
	}
	var in Input
	if err := json.Unmarshal(body, &in); err != nil {
		return Input{}, err
	}
	return in, nil
}

// blockOutput is the JSON shape Claude Code expects on stdout when a hook blocks.
type blockOutput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type allowWithContext struct {
	HookSpecificOutput hookSpecific `json:"hookSpecificOutput"`
}

type hookSpecific struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// WriteBlock emits the block JSON to w.
func WriteBlock(w io.Writer, reason string) error {
	return json.NewEncoder(w).Encode(blockOutput{Decision: "block", Reason: reason})
}

// WriteAllowWithOverride emits an allow with an additionalContext message
// surfacing the repo-level override reason loudly to Claude (and the human).
func WriteAllowWithOverride(w io.Writer, reason string) error {
	return json.NewEncoder(w).Encode(allowWithContext{
		HookSpecificOutput: hookSpecific{
			HookEventName:     "PreToolUse",
			AdditionalContext: "🔓 allowed by repo policy: " + reason,
		},
	})
}
