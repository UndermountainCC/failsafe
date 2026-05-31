// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package mode implements the configurable mode-source chain (spec §3.4).
// Mode is binary: "read" or "read & write". Sources are tried in the order
// declared in config; first to return a value wins.
package mode

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Source resolves to a mode value from some authority (env var, file, etc.).
type Source interface {
	// Resolve returns (value, true, nil) if the source has a value to contribute,
	// (_, false, nil) if it should be skipped (unset env, missing file, vars unset),
	// or (_, _, err) only on unexpected I/O errors that the operator should know about.
	Resolve(env map[string]string) (string, bool, error)

	// Writable returns true if this source can accept a write (toggle or set).
	// Env sources are not writable from a child process.
	Writable() bool

	// Path returns the resolved write target (after var expansion) when
	// Writable() is true. Returns empty string and ok=false if the source
	// cannot resolve in the current environment (e.g., the file's vars are unset).
	Path(env map[string]string) (string, bool)
}

// EnvSource reads from a single environment variable.
type EnvSource struct{ Name string }

func (e EnvSource) Resolve(env map[string]string) (string, bool, error) {
	v, ok := env[e.Name]
	if !ok || v == "" {
		return "", false, nil
	}
	return strings.TrimSpace(v), true, nil
}
func (EnvSource) Writable() bool                          { return false }
func (EnvSource) Path(_ map[string]string) (string, bool) { return "", false }

// FileSource reads from a file whose path may contain ${VAR} references.
// If any referenced var is unset, the source is skipped. The Pattern field
// is the pre-expansion template (e.g. "${HOME}/.claude/pane-mode/${WEZTERM_PANE}");
// the Path() method returns it after env substitution.
type FileSource struct {
	Pattern string `yaml:"path"`
}

func (f FileSource) Resolve(env map[string]string) (string, bool, error) {
	path, ok := f.Path(env)
	if !ok {
		return "", false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(string(body)), true, nil
}

func (FileSource) Writable() bool { return true }

func (f FileSource) Path(env map[string]string) (string, bool) {
	out, ok := expandVars(f.Pattern, env)
	return out, ok
}

// TTYSource is a writable per-controlling-terminal mode file. Without it, every
// plain shell (no WEZTERM_PANE/TMUX_PANE/… set) would share the single global
// mode file, so toggling write in one terminal would flip them all. Keying on
// the controlling tty gives each terminal window its own read/write mode while
// still resolving to a writable target.
//
// The id comes from the CONTROLLING terminal (/dev/tty), NOT stdin: the
// PreToolUse hook is invoked with hook JSON on stdin (a pipe), so stdin is not a
// tty — yet the hook still shares the shell's controlling terminal. Resolving
// via /dev/tty makes the interactive `toggle` and the piped hook agree on the
// same file. With no controlling terminal (headless/CI) the source is skipped
// and the chain falls through to the global file.
type TTYSource struct {
	Dir string                // pre-expansion dir template, e.g. "${HOME}/.config/failsafe"
	TTY func() (string, bool) // controlling-tty id resolver; nil → controllingTTY
}

func (t TTYSource) ttyID() (string, bool) {
	fn := t.TTY
	if fn == nil {
		fn = controllingTTY
	}
	return fn()
}

func (t TTYSource) Resolve(env map[string]string) (string, bool, error) {
	path, ok := t.Path(env)
	if !ok {
		return "", false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(string(body)), true, nil
}

func (TTYSource) Writable() bool { return true }

func (t TTYSource) Path(env map[string]string) (string, bool) {
	id, ok := t.ttyID()
	if !ok {
		return "", false
	}
	dir, ok := expandVars(t.Dir, env)
	if !ok {
		return "", false
	}
	return filepath.Join(dir, "tty-"+id), true
}

// controllingTTY returns a stable identifier for the process's controlling
// terminal, taken from the device id of the opened /dev/tty. ok=false when there
// is no controlling terminal (headless, CI, daemon).
func controllingTTY() (string, bool) {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return "", false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}
	return strconv.FormatUint(uint64(st.Rdev), 10), true
}

// expandVars performs ${VAR} substitution. Returns ok=false if any var is unset
// (so callers can skip the entire source rather than substituting empty strings).
func expandVars(in string, env map[string]string) (string, bool) {
	var out strings.Builder
	i := 0
	for i < len(in) {
		if i+1 < len(in) && in[i] == '$' && in[i+1] == '{' {
			end := strings.Index(in[i+2:], "}")
			if end == -1 {
				return "", false
			}
			name := in[i+2 : i+2+end]
			v, ok := env[name]
			if !ok || v == "" {
				return "", false
			}
			out.WriteString(v)
			i += 2 + end + 1
			continue
		}
		out.WriteByte(in[i])
		i++
	}
	return out.String(), true
}
