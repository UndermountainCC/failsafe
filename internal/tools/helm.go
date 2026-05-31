// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import "path/filepath"

func NewHelm() Tool { return &helmTool{} }

type helmTool struct{}

func (helmTool) Name() string          { return "helm" }
func (helmTool) Match(tok string) bool { return filepath.Base(tok) == "helm" }
func (helmTool) Enrichers() []string   { return nil }

var helmValueFlags = map[string]struct{}{
	"namespace": {}, "kube-context": {}, "kubeconfig": {}, "kube-apiserver": {},
	"kube-as-group": {}, "kube-as-user": {}, "kube-token": {},
	"registry-config": {}, "repository-cache": {}, "repository-config": {},
	"output": {}, "v": {},
	// Chart-customization flags. Values may be policy-relevant (e.g. a
	// repo allow_override may want to inspect what a chart sets), so we
	// must capture them rather than treating them as boolean — also
	// avoids swallowing the next argv as a positional.
	"set": {}, "set-string": {}, "set-file": {}, "set-json": {},
	"values": {}, "repo": {}, "version": {},
}

// helmShortToLong maps single-letter short forms to their long names. -f is
// the standard helm short for --values (file path).
var helmShortToLong = map[string]string{"n": "namespace", "o": "output", "f": "values"}

// helmSubverbsByVerb lists the subverb whitelist per verb. Bundled helm.rego
// keys off `input.subverb` for `helm repo <subverb>`, so extracting it is
// load-bearing for `helm repo list` to be allowed in read mode.
var helmSubverbsByVerb = map[string][]string{
	"repo":   {"add", "remove", "list", "update", "index", "remove-url"},
	"plugin": {"install", "list", "uninstall", "update"},
	"search": {"hub", "repo"},
}

func (helmTool) Parse(args []string) (Parsed, error) {
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
			if _, isVal := helmValueFlags[name]; isVal {
				if hasEq {
					out.Flags[name] = val
				} else if i+1 < len(args) {
					out.Flags[name] = args[i+1]
					i++
				}
			} else {
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
				if long, ok := helmShortToLong[short]; ok {
					out.Flags[long] = val
				} else {
					out.Flags[short] = val
				}
				i++
				continue
			}
			if len(body) == 1 {
				if long, ok := helmShortToLong[body]; ok {
					if _, isVal := helmValueFlags[long]; isVal && i+1 < len(args) {
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
			out.Flags[body] = true
			i++
			continue
		}
		if out.Verb == "" {
			out.Verb = a
			i++
			// Look up subverbs for compound verbs like `helm repo list`.
			if subs, ok := helmSubverbsByVerb[a]; ok && i < len(args) {
				next := args[i]
				for _, sv := range subs {
					if next == sv {
						out.Subverb = next
						i++
						break
					}
				}
			}
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
