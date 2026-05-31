// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runMCP feeds the lines in `requests` to MCP() (newline-terminated) and
// returns the response objects parsed from stdout. Each non-empty line in
// stdout is one JSON-RPC response.
func runMCP(t *testing.T, requests []string) (responses []map[string]any, stdoutRaw string) {
	t.Helper()
	var stdin bytes.Buffer
	for _, r := range requests {
		stdin.WriteString(r)
		stdin.WriteByte('\n')
	}
	var stdout, stderr bytes.Buffer
	code := MCP(&stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("MCP returned exit=%d, stderr=%q", code, stderr.String())
	}
	stdoutRaw = stdout.String()
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Fatalf("invalid JSON response %q: %v", string(line), err)
		}
		responses = append(responses, resp)
	}
	return responses, stdoutRaw
}

func TestMCP_InitializeReturnsProtocol(t *testing.T) {
	resps, _ := runMCP(t, []string{`{"jsonrpc":"2.0","id":1,"method":"initialize"}`})
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %+v", resps[0])
	}
	if got := result["protocolVersion"]; got != "2024-11-05" {
		t.Errorf("protocolVersion=%v, want 2024-11-05", got)
	}
	server, _ := result["serverInfo"].(map[string]any)
	if got := server["name"]; got != "guard" {
		t.Errorf("serverInfo.name=%v, want guard", got)
	}
	if got := server["version"]; got != "1.0.0" {
		t.Errorf("serverInfo.version=%v, want 1.0.0", got)
	}
}

func TestMCP_ToolsListReturnsTwoTools(t *testing.T) {
	resps, _ := runMCP(t, []string{`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`})
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	result, _ := resps[0]["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("want 2 tools, got %d", len(tools))
	}

	wantCheckDesc := "Check the current infrastructure guard mode for this pane. Returns 'read' (destructive infra commands blocked) or 'read & write' (all commands allowed). Check this before attempting any kubectl/terraform/helm/aws mutating command."
	wantToggleDesc := "Toggle the infrastructure guard mode for this pane between 'read' and 'read & write'. IMPORTANT: When you need to run a destructive infra command and the mode is 'read', you MUST call this tool and ask the user to approve the toggle. Do NOT try alternative approaches, workarounds, or suggest the user run the command manually. Stop and request the toggle."

	byName := map[string]map[string]any{}
	for _, t0 := range tools {
		m, _ := t0.(map[string]any)
		name, _ := m["name"].(string)
		byName[name] = m
	}
	check, ok := byName["check_mode"]
	if !ok {
		t.Fatalf("check_mode missing; got names=%v", keysOf(byName))
	}
	if got := check["description"]; got != wantCheckDesc {
		t.Errorf("check_mode description mismatch:\n got: %v\nwant: %v", got, wantCheckDesc)
	}
	toggle, ok := byName["toggle_mode"]
	if !ok {
		t.Fatalf("toggle_mode missing; got names=%v", keysOf(byName))
	}
	if got := toggle["description"]; got != wantToggleDesc {
		t.Errorf("toggle_mode description mismatch:\n got: %v\nwant: %v", got, wantToggleDesc)
	}
	// Schema shape: empty object schema with required=[].
	for _, m := range []map[string]any{check, toggle} {
		schema, _ := m["inputSchema"].(map[string]any)
		if schema["type"] != "object" {
			t.Errorf("inputSchema.type=%v, want object (tool=%v)", schema["type"], m["name"])
		}
	}
}

// withIsolatedEnv sets HOME to a temp dir and points all known pane env vars
// at empty/known values; returns the temp HOME dir.
func withIsolatedEnv(t *testing.T, paneVar, paneID string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Clear all pane vars so only the one we want is set.
	for _, v := range []string{"WEZTERM_PANE", "TMUX_PANE", "ITERM_SESSION_ID", "KITTY_WINDOW_ID", "CLAUDE_SESSION_ID", "FAILSAFE_MODE"} {
		t.Setenv(v, "")
		_ = os.Unsetenv(v)
	}
	if paneVar != "" {
		t.Setenv(paneVar, paneID)
	}
	return home
}

func TestMCP_CheckModeReadsChain(t *testing.T) {
	home := withIsolatedEnv(t, "ITERM_SESSION_ID", "session-abc")

	// Write the mode file the chain will hit.
	modeFile := filepath.Join(home, ".claude", "pane-mode", "session-abc")
	if err := os.MkdirAll(filepath.Dir(modeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modeFile, []byte("read & write"), 0o644); err != nil {
		t.Fatal(err)
	}

	resps, _ := runMCP(t, []string{`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"check_mode"}}`})
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	text := extractTextContent(t, resps[0])
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("content text not JSON: %v (%q)", err, text)
	}
	if payload["mode"] != "read & write" {
		t.Errorf("mode=%v, want 'read & write'", payload["mode"])
	}
	if payload["label"] != "rw" {
		t.Errorf("label=%v, want rw", payload["label"])
	}
	if payload["pane_id"] != "session-abc" {
		t.Errorf("pane_id=%v, want session-abc", payload["pane_id"])
	}
}

func TestMCP_ToggleModeFlipsAndReturnsOldNew(t *testing.T) {
	home := withIsolatedEnv(t, "ITERM_SESSION_ID", "session-xyz")

	// Start in "read" (no file means default "read"; create it explicitly so
	// the toggle has a deterministic source to write back to).
	modeFile := filepath.Join(home, ".claude", "pane-mode", "session-xyz")
	if err := os.MkdirAll(filepath.Dir(modeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modeFile, []byte("read"), 0o644); err != nil {
		t.Fatal(err)
	}

	resps, _ := runMCP(t, []string{`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"toggle_mode"}}`})
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	text := extractTextContent(t, resps[0])
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("content text not JSON: %v (%q)", err, text)
	}
	if payload["old"] != "read" {
		t.Errorf("old=%v, want read", payload["old"])
	}
	if payload["new"] != "read & write" {
		t.Errorf("new=%v, want 'read & write'", payload["new"])
	}
	if payload["pane_id"] != "session-xyz" {
		t.Errorf("pane_id=%v, want session-xyz", payload["pane_id"])
	}

	// Verify the file was actually flipped.
	body, err := os.ReadFile(modeFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "read & write" {
		t.Errorf("mode file=%q, want 'read & write'", body)
	}
}

func TestMCP_UnknownMethodReturnsError(t *testing.T) {
	resps, _ := runMCP(t, []string{`{"jsonrpc":"2.0","id":5,"method":"garbage"}`})
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	errObj, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error: %+v", resps[0])
	}
	if code, _ := errObj["code"].(float64); code != -32601 {
		t.Errorf("error.code=%v, want -32601", errObj["code"])
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "garbage") {
		t.Errorf("error.message=%q, want it to mention 'garbage'", msg)
	}
}

func TestMCP_NotificationsInitializedNoResponse(t *testing.T) {
	_, raw := runMCP(t, []string{`{"jsonrpc":"2.0","method":"notifications/initialized"}`})
	if raw != "" {
		t.Errorf("expected empty stdout, got %q", raw)
	}
}

// extractTextContent pulls result.content[0].text from a tools/call response.
func extractTextContent(t *testing.T, resp map[string]any) string {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %+v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content: %+v", result)
	}
	first, _ := content[0].(map[string]any)
	if first["type"] != "text" {
		t.Fatalf("content[0].type=%v, want text", first["type"])
	}
	text, _ := first["text"].(string)
	return text
}

func keysOf(m map[string]map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
