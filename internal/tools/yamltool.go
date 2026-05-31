// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// flagDef mirrors the YAML schema's flag shape.
type flagDef struct {
	Long       string `yaml:"long"`
	Short      string `yaml:"short"`
	TakesValue bool   `yaml:"takes_value"`
	Repeated   bool   `yaml:"repeated"`
	Style      string `yaml:"style"` // "gnu" (default), "gnu_short", "short"
}

type verbDef struct {
	Flags    []flagDef `yaml:"flags"`
	Subverbs []string  `yaml:"subverbs"`
	Enrich   []string  `yaml:"enrich"`
}

type yamlSchema struct {
	Name          string             `yaml:"name"`
	Match         []string           `yaml:"match"`
	EnvPrefix     *bool              `yaml:"env_prefix"`
	GlobalFlags   []flagDef          `yaml:"global_flags"`
	Verbs         map[string]verbDef `yaml:"verbs"`
	Enrich        []string           `yaml:"enrich"`
	CombineShorts bool               `yaml:"combine_shorts"`
}

// LoadYAMLTool parses a tool YAML and returns a Tool.
func LoadYAMLTool(r io.Reader) (Tool, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	var s yamlSchema
	if err := yaml.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	if s.Name == "" {
		return nil, fmt.Errorf("tool yaml: name is required")
	}
	if len(s.Match) == 0 {
		return nil, fmt.Errorf("tool yaml: match is required (at least one entry)")
	}
	return &yamlTool{schema: s}, nil
}

type yamlTool struct{ schema yamlSchema }

func (t *yamlTool) Name() string { return t.schema.Name }

func (t *yamlTool) Match(tok string) bool {
	base := filepath.Base(tok)
	for _, m := range t.schema.Match {
		if base == m {
			return true
		}
	}
	return false
}

func (t *yamlTool) Enrichers() []string { return append([]string(nil), t.schema.Enrich...) }

// Parse implements the flag-skipping argv walker described in spec §3.3 + §5.2.
func (t *yamlTool) Parse(args []string) (Parsed, error) {
	out := Parsed{Flags: map[string]interface{}{}}

	flagsByLong := map[string]flagDef{}
	flagsByShort := map[string]flagDef{}
	for _, f := range t.schema.GlobalFlags {
		flagsByLong[f.Long] = f
		if f.Short != "" {
			flagsByShort[f.Short] = f
		}
	}

	i := 0
	for i < len(args) {
		a := args[i]

		// "--" terminates flag parsing
		if a == "--" {
			i++
			break
		}

		// gnu_short: single-dash long flag (terraform-style -chdir, -chdir=val)
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") {
			if name, val, gnu := matchGNUShort(a, flagsByLong); gnu {
				if val != "" {
					out.Flags[name] = val
					i++
					continue
				}
				// space form: -chdir VAL — only consume next when this
				// flag takes a value, otherwise we'd swallow the verb.
				def := flagsByLong[name]
				if def.TakesValue && i+1 < len(args) {
					out.Flags[name] = args[i+1]
					i += 2
					continue
				}
				out.Flags[name] = true
				i++
				continue
			}
		}

		// --long or --long=val
		if strings.HasPrefix(a, "--") {
			name, val, hasEq := strings.Cut(a[2:], "=")
			if def, ok := flagsByLong[name]; ok {
				if def.TakesValue {
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
			// unknown long flag: skip just this token (don't consume next).
			// Unknown flag arity is unknowable from the YAML schema; consuming
			// the next argv would misparse common boolean flags like
			// `terraform --no-color destroy` (next token is the verb, not a
			// value). Storing as bool=true causes the next token to be picked
			// up by the normal verb/positional handling below — which under
			// bundled rego's `input.verb != ""` gate correctly fail-closes on
			// an unknown verb.
			out.Flags[name] = true
			i++
			continue
		}

		// -x, -x=val, or combined -xy[z]
		if strings.HasPrefix(a, "-") && len(a) >= 2 {
			body := a[1:]
			// -X=val
			if eq := strings.Index(body, "="); eq != -1 {
				short := body[:eq]
				val := body[eq+1:]
				if def, ok := flagsByShort[short]; ok && def.TakesValue {
					out.Flags[def.Long] = val
				} else {
					out.Flags[short] = val
				}
				i++
				continue
			}
			// -X (single short)
			if len(body) == 1 {
				if def, ok := flagsByShort[body]; ok {
					if def.TakesValue {
						if i+1 < len(args) {
							out.Flags[def.Long] = args[i+1]
							i += 2
							continue
						}
					}
					out.Flags[def.Long] = true
					i++
					continue
				}
				out.Flags[body] = true
				i++
				continue
			}
			// combined shorts (-it) — only when tool opts in
			if t.schema.CombineShorts {
				for _, c := range body {
					s := string(c)
					if def, ok := flagsByShort[s]; ok && !def.TakesValue {
						out.Flags[def.Long] = true
					} else {
						out.Flags[s] = true
					}
				}
				i++
				continue
			}
			// fall through: treat as single flag-ish token
			out.Flags[body] = true
			i++
			continue
		}

		// non-flag token: this is the verb (or, after verb, positional)
		if out.Verb == "" {
			out.Verb = a
			i++
			// look up subverbs
			if vd, ok := t.schema.Verbs[a]; ok && len(vd.Subverbs) > 0 && i < len(args) {
				next := args[i]
				for _, sv := range vd.Subverbs {
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
	// remaining args after "--" are all positional
	for ; i < len(args); i++ {
		out.Positional = append(out.Positional, args[i])
	}
	return out, nil
}

// matchGNUShort returns (name, val, true) if the token is a recognized
// gnu_short flag (single-dash long, like terraform's -chdir or -chdir=val).
func matchGNUShort(tok string, flagsByLong map[string]flagDef) (string, string, bool) {
	body := tok[1:]
	name, val, hasEq := strings.Cut(body, "=")
	if def, ok := flagsByLong[name]; ok && def.Style == "gnu_short" {
		if hasEq {
			return name, val, true
		}
		return name, "", true
	}
	return "", "", false
}
