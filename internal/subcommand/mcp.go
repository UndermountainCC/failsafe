// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/UndermountainCC/failsafe/internal/mode"
)

// MCP runs the stdio JSON-RPC server. Reads requests from stdin one per line,
// writes responses to stdout one per line, errors to stderr. Returns 0 on EOF.
//
// Ports plugin/mcp/guard_mcp.py verbatim — same protocol, same tool descriptions.
// The only behavioral difference is that this version inherits the full mode
// chain (defaultModeChain), so it works on iTerm/tmux/kitty in addition to
// WezTerm where the Python version was a no-op.
func MCP(stdin io.Reader, stdout, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	// MCP tools/call payloads can be larger than the default 64KiB; bump to
	// 1 MiB to be safe (Python uses unbuffered line reads, no equivalent cap).
	const maxLine = 1 << 20
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req map[string]any
		if err := json.Unmarshal(line, &req); err != nil {
			// Match Python: silently skip malformed lines.
			continue
		}
		resp, ok := handleMCPRequest(req)
		if !ok {
			continue // notification — no response
		}
		body, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(stderr, "failsafe mcp: marshal response: %v\n", err)
			continue
		}
		body = append(body, '\n')
		if _, err := stdout.Write(body); err != nil {
			fmt.Fprintf(stderr, "failsafe mcp: write response: %v\n", err)
			return 1
		}
		if f, ok := stdout.(interface{ Sync() error }); ok {
			_ = f.Sync()
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "failsafe mcp: read stdin: %v\n", err)
		return 1
	}
	return 0
}

// mcpTools is the TOOLS list. The description strings are the prompt text
// Claude reads to decide when to invoke these tools — they are CONTRACT, not
// docs. Copied verbatim from plugin/mcp/guard_mcp.py.
var mcpTools = []map[string]any{
	{
		"name":        "check_mode",
		"description": "Check the current infrastructure guard mode for this pane. Returns 'read' (destructive infra commands blocked) or 'read & write' (all commands allowed). Check this before attempting any kubectl/terraform/helm/aws mutating command.",
		"inputSchema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []any{},
		},
	},
	{
		"name":        "toggle_mode",
		"description": "Toggle the infrastructure guard mode for this pane between 'read' and 'read & write'. IMPORTANT: When you need to run a destructive infra command and the mode is 'read', you MUST call this tool and ask the user to approve the toggle. Do NOT try alternative approaches, workarounds, or suggest the user run the command manually. Stop and request the toggle.",
		"inputSchema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []any{},
		},
	},
}

// handleMCPRequest dispatches one JSON-RPC request. Returns (response, true)
// for a request that should produce a response, or (nil, false) for a
// notification (no response).
func handleMCPRequest(req map[string]any) (map[string]any, bool) {
	method, _ := req["method"].(string)
	id := req["id"] // may be nil for notifications

	switch method {
	case "initialize":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "guard", "version": "1.0.0"},
			},
		}, true

	case "notifications/initialized":
		return nil, false

	case "tools/list":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"tools": mcpTools},
		}, true

	case "tools/call":
		params, _ := req["params"].(map[string]any)
		toolName, _ := params["name"].(string)
		return handleToolCall(id, toolName), true
	}

	if len(method) >= len("notifications/") && method[:len("notifications/")] == "notifications/" {
		return nil, false
	}
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    -32601,
			"message": "Unknown method: " + method,
		},
	}, true
}

func handleToolCall(id any, name string) map[string]any {
	chain := defaultModeChain()
	env := EnvFromOS()

	switch name {
	case "check_mode":
		current, _, _ := chain.Resolve(env)
		paneID := resolvePaneID(chain, env)
		text, _ := json.Marshal(map[string]any{
			"mode":    current,
			"pane_id": paneID,
			"label":   labelFor(current),
		})
		return mcpToolResult(id, string(text))

	case "toggle_mode":
		old, _, _ := chain.Resolve(env)
		paneID := resolvePaneID(chain, env)
		newVal := "read & write"
		if old == "read & write" {
			newVal = "read"
		}
		_, path, ok := chain.FirstWritable(env)
		if !ok {
			// Match Python: can't toggle without a writable target — return old/old.
			text, _ := json.Marshal(map[string]any{
				"old":     old,
				"new":     old,
				"pane_id": paneID,
			})
			return mcpToolResult(id, string(text))
		}
		if err := atomicWrite(path, []byte(newVal)); err != nil {
			return map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]any{
					"code":    -32000,
					"message": "toggle_mode: write failed: " + err.Error(),
				},
			}
		}
		text, _ := json.Marshal(map[string]any{
			"old":     old,
			"new":     newVal,
			"pane_id": paneID,
		})
		return mcpToolResult(id, string(text))
	}

	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    -32601,
			"message": "Unknown tool: " + name,
		},
	}
}

func mcpToolResult(id any, text string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": text},
			},
		},
	}
}

func labelFor(modeVal string) string {
	if modeVal == "read & write" {
		return "rw"
	}
	return "r"
}

// resolvePaneID picks the pane identifier to surface in tool responses.
// Walks the mode chain in declaration order and returns the trailing path
// component of the first FileSource whose Path() expands successfully — this
// matches the same precedence the chain uses to resolve mode (WEZTERM_PANE >
// TMUX_PANE > ITERM_SESSION_ID > KITTY_WINDOW_ID > CLAUDE_SESSION_ID).
// Returns "(unset)" if no source resolves (e.g., running outside any terminal).
func resolvePaneID(chain *mode.Chain, env map[string]string) string {
	for _, s := range chain.Sources {
		fs, ok := s.(mode.FileSource)
		if !ok {
			continue
		}
		path, ok := fs.Path(env)
		if !ok {
			continue
		}
		base := filepath.Base(path)
		if base == "" || base == "." || base == "/" {
			continue
		}
		// Skip the global fallback source (~/.config/failsafe/mode), whose
		// trailing component is the literal "mode" — not a pane identifier.
		if fs.Pattern == "${HOME}/.config/failsafe/mode" {
			continue
		}
		return base
	}
	return "(unset)"
}
