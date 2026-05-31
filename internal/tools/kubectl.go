// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"path/filepath"
)

// NewKubectl returns the Go-coded kubectl Tool. The flag list mirrors
// kubectl's persistent flags (the ones that take values); subcommand flags
// don't matter because policies key off the verb, not on per-verb flags.
func NewKubectl() Tool {
	return &kubectlTool{}
}

type kubectlTool struct{}

func (kubectlTool) Name() string { return "kubectl" }

func (kubectlTool) Match(tok string) bool {
	return filepath.Base(tok) == "kubectl"
}

func (kubectlTool) Enrichers() []string { return []string{"kubectl_context"} }

var kubectlValueFlags = map[string]struct{}{
	"namespace": {}, "context": {}, "kubeconfig": {}, "cluster": {}, "user": {},
	"token": {}, "server": {}, "as": {}, "as-group": {}, "cache-dir": {},
	"certificate-authority": {}, "client-certificate": {}, "client-key": {},
	"request-timeout": {}, "v": {}, "log-flush-frequency": {},
	"output": {}, "selector": {}, "filename": {}, "field-selector": {},
	"profile": {}, "profile-output": {}, "tls-server-name": {}, "match-server-version": {},
	// kubectl's --dry-run is a STRING flag (none|client|server). Bundled
	// policy keys off `flags["dry-run"] in {"client", "server", ...}` to
	// allow the command, so the parser MUST capture the value, not bool.
	// `--dry-run=server` and `--dry-run server` both work post-fix.
	"dry-run": {},
}

var kubectlShortToLong = map[string]string{
	"n": "namespace", "s": "server", "o": "output", "l": "selector", "f": "filename", "v": "v",
}

func (kubectlTool) Parse(args []string) (Parsed, error) {
	out := Parsed{Flags: map[string]interface{}{}}
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if len(a) >= 2 && a[:2] == "--" {
			name, val, hasEq := splitOnce(a[2:], "=")
			if _, isVal := kubectlValueFlags[name]; isVal {
				if hasEq {
					out.Flags[name] = val
				} else if i+1 < len(args) {
					out.Flags[name] = args[i+1]
					i++
				}
			} else {
				// Unknown long flag: stored as bool=true and we DON'T consume the next token.
				// This is the conservative reading: we don't know whether the flag takes a
				// value, and assuming yes could mis-parse `kubectl --foo bar get pods`
				// (where bar is the verb). Leaving the next token to be picked up by
				// verb/positional handling matches the bash guard.sh's behavior.
				out.Flags[name] = true
			}
			i++
			continue
		}
		if len(a) >= 2 && a[0] == '-' {
			body := a[1:]
			if eq := indexByte(body, '='); eq != -1 {
				short := body[:eq]
				val := body[eq+1:]
				if long, ok := kubectlShortToLong[short]; ok {
					out.Flags[long] = val
				} else {
					out.Flags[short] = val
				}
				i++
				continue
			}
			if len(body) == 1 {
				if long, ok := kubectlShortToLong[body]; ok {
					if _, isVal := kubectlValueFlags[long]; isVal && i+1 < len(args) {
						out.Flags[long] = args[i+1]
						i += 2
						continue
					}
					out.Flags[long] = true
					i++
					continue
				}
				out.Flags[body] = true
				i++
				continue
			}
			// combined shorts: -it, -itx, etc. Treat each char as a boolean flag.
			for j := 0; j < len(body); j++ {
				short := string(body[j])
				if long, ok := kubectlShortToLong[short]; ok {
					out.Flags[long] = true
				} else {
					out.Flags[short] = true
				}
			}
			i++
			continue
		}
		// non-flag: verb (or positional after verb)
		if out.Verb == "" {
			out.Verb = a
			i++
			continue
		}
		out.Positional = append(out.Positional, a)
		i++
	}
	for ; i < len(args); i++ {
		out.Positional = append(out.Positional, args[i])
	}
	return out, nil
}
