// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package shellparser parses a shell command string with mvdan.cc/sh and
// extracts every "effective call" — argv-level invocations the shell would
// actually run. This is the entry point of the hot path; downstream
// (registry, policy) operates on []EffectiveCall.
//
// We refuse to analyze constructs whose effects we cannot safely model
// (subshells, command substitution, eval, function definitions/calls,
// loops, conditionals). The refuse reason is reported as a separate output;
// the engine treats refusal as block.
package shellparser

import (
	"fmt"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// DynamicMarker is the placeholder we substitute for argv tokens we cannot
// statically resolve (var expansion, command substitution, process subst,
// arithmetic). Emitting this rather than refusing the whole walk lets the
// registry+policy layer make the gating decision: unregistered tools allow,
// and bundled rego treats unknown verbs as non-matching read_verbs (i.e.
// blocks). Dynamic HEAD still refuses — a dynamic command name could be
// anything and there's no policy gate that recovers from that.
const DynamicMarker = "<dynamic>"

// EffectiveCall is one argv-level invocation extracted from a shell AST.
type EffectiveCall struct {
	Name string
	Args []string
	Env  map[string]string
	Cwd  string
	Line int
	Col  int
	// UncertainCwd is true when this call followed an ambiguous cd
	// (semicolon, OrStmt, or post-AndStmt boundary). The walker emits
	// such calls so the hook layer can decide: registered infra tools
	// should refuse, non-infra calls (echo, ls, gh, etc.) can proceed.
	// The Cwd field on these calls reflects whatever the walker last
	// tracked, but at runtime the actual cwd may be the original (if cd
	// failed) or the cd target (if cd succeeded) — they're interchangeable
	// only for cwd-insensitive commands.
	UncertainCwd bool
}

// Extract parses cmd into a shell AST and returns every effective call.
func Extract(cmd string) (calls []EffectiveCall, refuse string, err error) {
	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return nil, "", fmt.Errorf("shell parse: %w", err)
	}
	w := walker{}
	for _, stmt := range file.Stmts {
		w.walkStmt(stmt)
		if w.refuse != "" {
			return w.calls, w.refuse, nil
		}
	}
	return w.calls, "", nil
}

type walker struct {
	calls          []EffectiveCall
	refuse         string
	curCwd         string
	inAndChain     bool
	cdCount        int
	sawAmbiguousCd bool
}

var shellEvalBuiltins = map[string]bool{
	"eval": true, "source": true, ".": true, "exec": true,
}

var shellStateBuiltins = map[string]bool{
	"export": true, "unset": true, "set": true, "alias": true, "unalias": true,
	"declare": true, "typeset": true, "local": true, "readonly": true,
	"shopt": true, "ulimit": true, "umask": true, "trap": true,
	"shift": true, "break": true, "continue": true, "return": true,
	"bind": true, "complete": true, "compopt": true,
	"pushd": true, "popd": true, "dirs": true, "hash": true,
	"enable": true, "disable": true, "history": true, "fc": true,
	"jobs": true, "fg": true, "bg": true, "wait": true, "caller": true,
}

var dashCWrappers = map[string]bool{"sh": true, "bash": true}

var transparentWrappers = map[string]bool{
	"command": true, "builtin": true,
	"nice": true, "nohup": true, "time": true,
	"ionice": true, "chrt": true, "taskset": true,
}

// blanketRefuseWrappers lists wrappers we always refuse — kept as a map for
// defense-in-depth so future strictly-refuse wrappers can be added back.
var blanketRefuseWrappers = map[string]bool{}

// xargsBoolFlags lists xargs flags that are boolean (no value, semantics-safe
// to ignore for our purposes). Any flag NOT in this set causes the peel to
// refuse — value-taking flags like -I, -n, -P fundamentally change how xargs
// invokes the inner command in ways we can't safely model.
var xargsBoolFlags = map[string]bool{
	"-0":                true, // --null
	"--null":            true,
	"-r":                true, // --no-run-if-empty
	"--no-run-if-empty": true,
	"-t":                true, // trace
	"-p":                true, // interactive prompt
	"-x":                true, // exit on error
	"--verbose":         true,
	"--interactive":     true,
}

var knownShellNames = map[string]bool{
	"zsh": true, "fish": true, "ksh": true, "dash": true, "csh": true,
	"tcsh": true, "ash": true, "mksh": true, "yash": true, "rc": true,
	"elvish": true, "xonsh": true,
}

func (w *walker) walkStmt(s *syntax.Stmt) {
	if s == nil || w.refuse != "" {
		return
	}
	if s.Background || s.Coprocess {
		w.refuse = "background or coprocess statement"
		return
	}
	for _, r := range s.Redirs {
		switch r.Op {
		case syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
			w.refuse = "heredoc redirection (`<<`/`<<-`/`<<<`) not analyzable"
			return
		case syntax.RdrIn:
			w.refuse = "input redirection (`<`) hides the command's actual input from policy"
			return
		case syntax.DplIn:
			w.refuse = "fd-duplicating input redirection (`<&`) not modeled"
			return
		}
	}
	w.walkCmd(s, s.Cmd)
}

func (w *walker) walkCmd(stmt *syntax.Stmt, c syntax.Command) {
	if w.refuse != "" {
		return
	}
	switch v := c.(type) {
	case *syntax.CallExpr:
		w.walkCallExpr(stmt, v)
	case *syntax.BinaryCmd:
		switch v.Op {
		case syntax.AndStmt:
			prevInAnd := w.inAndChain
			cdsBefore := w.cdCount
			w.inAndChain = true
			w.walkStmt(v.X)
			w.inAndChain = prevInAnd
			w.walkStmt(v.Y)
			if !prevInAnd && w.cdCount > cdsBefore {
				w.sawAmbiguousCd = true
			}
		case syntax.OrStmt, syntax.Pipe, syntax.PipeAll:
			previous := w.curCwd
			w.walkStmt(v.X)
			w.curCwd = previous
			w.walkStmt(v.Y)
		default:
			w.refuse = fmt.Sprintf("unsupported binary op: %v", v.Op)
		}
	case *syntax.Subshell:
		w.refuse = "subshell ( ... ) — side-effect scoping not modeled"
	case *syntax.Block:
		for _, st := range v.Stmts {
			w.walkStmt(st)
		}
	case *syntax.IfClause, *syntax.WhileClause, *syntax.ForClause,
		*syntax.CaseClause, *syntax.FuncDecl, *syntax.TestClause:
		w.refuse = fmt.Sprintf("control-flow construct (%T) not analyzable", v)
	case *syntax.LetClause, *syntax.DeclClause:
		w.refuse = "declaration/assignment construct not analyzable"
	default:
		w.refuse = fmt.Sprintf("unrecognized command type %T", v)
	}
}

func resolveCwd(prior, target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	if prior == "" {
		return target
	}
	return filepath.Clean(filepath.Join(prior, target))
}

func (w *walker) walkCallExpr(stmt *syntax.Stmt, c *syntax.CallExpr) {
	env := map[string]string{}
	for _, a := range c.Assigns {
		if a.Naked || a.Append || a.Index != nil {
			w.refuse = "non-trivial assignment in command prefix"
			return
		}
		if a.Value == nil {
			continue
		}
		if reason := dangerousUnquoted(a.Value); reason != "" {
			w.refuse = "env-prefix " + a.Name.Value + ": " + reason
			return
		}
		val, ok := flattenWord(a.Value)
		if !ok {
			w.refuse = "dynamic env-prefix value (var expansion / command substitution)"
			return
		}
		if isStartupSourcingEnvVar(a.Name.Value) {
			w.refuse = "env-prefix " + a.Name.Value + " can source shell startup files; not analyzable"
			return
		}
		env[a.Name.Value] = val
	}

	if len(c.Args) == 0 {
		return
	}

	var flat []string
	for i, w2 := range c.Args {
		if reason := dangerousUnquoted(w2); reason != "" {
			w.refuse = reason
			return
		}
		s, ok := flattenWord(w2)
		if !ok {
			if i == 0 {
				w.refuse = "dynamic head (command name from var expansion / command substitution) — head must be statically determinable"
				return
			}
			flat = append(flat, DynamicMarker)
			continue
		}
		flat = append(flat, s)
	}

	for depth := 0; depth < 8; depth++ {
		if len(flat) == 0 {
			w.refuse = "wrapper resolved to no command"
			return
		}
		head := filepath.Base(flat[0])

		if head == "cd" {
			if len(flat) == 1 {
				w.refuse = "bare `cd` (resolves to $HOME) — target not statically known"
				return
			}
			if len(flat) > 2 {
				w.refuse = "cd with multiple args not supported"
				return
			}
			target := flat[1]
			if target == "-" {
				w.refuse = "`cd -` (resolves to $OLDPWD) — target not statically known"
				return
			}
			if target == "~" || strings.HasPrefix(target, "~") {
				w.refuse = "`cd ~`/`cd ~user` — tilde expansion not modeled"
				return
			}
			if cdpath, hasCdpath := env["CDPATH"]; hasCdpath && cdpath != "" {
				w.refuse = "cd with explicit CDPATH=" + cdpath + " — target resolution depends on $CDPATH and isn't statically determinable"
				return
			}
			if !filepath.IsAbs(target) &&
				!strings.HasPrefix(target, "./") &&
				!strings.HasPrefix(target, "../") &&
				target != "." && target != ".." {
				w.refuse = "cd to bare-relative target `" + target + "` — could be $CDPATH-resolved at runtime; use `./" + target + "` or an absolute path"
				return
			}
			w.cdCount++
			if !w.inAndChain {
				w.sawAmbiguousCd = true
			}
			w.curCwd = resolveCwd(w.curCwd, target)
			return
		}

		if shellEvalBuiltins[head] {
			w.refuse = head + " — dynamic shell evaluation not analyzable"
			return
		}
		if shellStateBuiltins[head] {
			w.refuse = head + " — shell state mutation not modeled (refused so policy sees the right environment)"
			return
		}
		if blanketRefuseWrappers[head] {
			w.refuse = head + " wrapper has dynamic semantics; not analyzable"
			return
		}
		if isUnknownDashCWrapper(flat) {
			w.refuse = "unsupported shell wrapper with -c (only sh and bash recognized): " + flat[0]
			return
		}
		if dashCWrappers[head] {
			w.unwrapDashC(stmt, head, flat[1:], env)
			return
		}
		if head == "env" {
			newFlat, newEnv, ok := w.peelEnv(flat[1:], env)
			if !ok {
				return
			}
			flat = newFlat
			env = newEnv
			continue
		}
		if transparentWrappers[head] {
			newFlat, ok := w.peelTransparent(head, flat[1:])
			if !ok {
				return
			}
			flat = newFlat
			continue
		}
		// xargs: peel into inner CMD if flags are all boolean-safe. Otherwise
		// refuse. Value-taking flags like -I, -n, -P change inner-command
		// argv shape in ways we can't model.
		if head == "xargs" {
			newFlat, ok := w.peelXargs(flat[1:])
			if !ok {
				return
			}
			flat = newFlat
			continue
		}

		line, col := positionOf(stmt)
		w.calls = append(w.calls, EffectiveCall{
			Name: flat[0], Args: flat[1:], Env: env,
			Cwd: w.curCwd, Line: line, Col: col,
			UncertainCwd: w.sawAmbiguousCd,
		})
		return
	}
	w.refuse = "wrapper nesting too deep"
}

func (w *walker) peelEnv(rest []string, env map[string]string) ([]string, map[string]string, bool) {
	extraEnv := map[string]string{}
	for k, v := range env {
		extraEnv[k] = v
	}
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if strings.HasPrefix(a, "-") {
			w.refuse = "env wrapper with options not supported: " + a
			return nil, nil, false
		}
		if eq := strings.IndexByte(a, '='); eq > 0 && isEnvName(a[:eq]) {
			if isStartupSourcingEnvVar(a[:eq]) {
				w.refuse = "env wrapper sets " + a[:eq] + " which can source shell startup files; not analyzable"
				return nil, nil, false
			}
			extraEnv[a[:eq]] = a[eq+1:]
			continue
		}
		return rest[i:], extraEnv, true
	}
	w.refuse = "env wrapper with no command"
	return nil, nil, false
}

// peelXargs strips xargs and its boolean-only flags, returning the inner
// command + its literal args. Stdin-derived args are added as a single
// DynamicMarker placeholder at the end of the inner argv (xargs appends
// stdin items as args to the inner CMD). Refuses on any non-boolean xargs
// flag — those (especially -I, -n) change inner-command argv shape in ways
// we can't safely model.
func (w *walker) peelXargs(rest []string) ([]string, bool) {
	i := 0
	for ; i < len(rest); i++ {
		a := rest[i]
		if !strings.HasPrefix(a, "-") {
			break
		}
		// Check the literal token (e.g., "-0", "--verbose"). If it's not
		// exactly in our boolean allowlist, refuse — could be a value-taking
		// flag we can't safely skip. This also catches `--replace=foo` and
		// `--max-args=5` because those literal tokens aren't in the allowlist.
		if !xargsBoolFlags[a] {
			w.refuse = "xargs flag " + a + " not in safe-peel allowlist (value-taking or unknown)"
			return nil, false
		}
	}
	if i >= len(rest) {
		w.refuse = "xargs with no inner command"
		return nil, false
	}
	// Append a placeholder for stdin-fed args. If xargs is invoked with
	// literal args after CMD (e.g., `xargs CMD ARG1 ARG2`), those literal
	// args precede the stdin items. So:
	//   inner argv = CMD + literal-args + DynamicMarker
	inner := append([]string{}, rest[i:]...)
	inner = append(inner, DynamicMarker)
	return inner, true
}

func (w *walker) peelTransparent(name string, rest []string) ([]string, bool) {
	for j := 0; j < len(rest); j++ {
		a := rest[j]
		if strings.HasPrefix(a, "-") {
			w.refuse = name + " wrapper with options not supported (got flag: " + a + ")"
			return nil, false
		}
		return rest[j:], true
	}
	w.refuse = name + " wrapper with no command"
	return nil, false
}

func (w *walker) unwrapDashC(stmt *syntax.Stmt, head string, rest []string, env map[string]string) {
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if a == "-c" {
			if i+1 >= len(rest) {
				w.refuse = head + " -c with no string argument"
				return
			}
			if i+2 < len(rest) {
				w.refuse = head + " -c with positional args after STRING not supported"
				return
			}
			inner := rest[i+1]
			subCalls, subRefuse, err := Extract(inner)
			if err != nil {
				w.refuse = "inner " + head + " parse failed: " + err.Error()
				return
			}
			if subRefuse != "" {
				w.refuse = "inside " + head + " -c: " + subRefuse
				return
			}
			for _, sc := range subCalls {
				if sc.Env == nil {
					sc.Env = map[string]string{}
				}
				for k, v := range env {
					if _, set := sc.Env[k]; !set {
						sc.Env[k] = v
					}
				}
				sc.Cwd = composeCwd(w.curCwd, sc.Cwd)
				if w.sawAmbiguousCd {
					sc.UncertainCwd = true
				}
				w.calls = append(w.calls, sc)
			}
			return
		}
		if a == "-x" {
			continue
		}
		w.refuse = head + " invocation without -c not supported (got: " + a + ")"
		return
	}
	w.refuse = head + " invocation without -c not supported"
}

func composeCwd(outer, inner string) string {
	if inner == "" {
		return outer
	}
	if filepath.IsAbs(inner) {
		return inner
	}
	if outer == "" {
		return inner
	}
	return filepath.Clean(filepath.Join(outer, inner))
}

func isStartupSourcingEnvVar(name string) bool {
	switch name {
	case "BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "PROMPT_COMMAND", "PS0":
		return true
	}
	return false
}

func dangerousUnquoted(w *syntax.Word) string {
	for i, p := range w.Parts {
		lit, ok := p.(*syntax.Lit)
		if !ok {
			continue
		}
		if strings.ContainsAny(lit.Value, "*?[{") {
			return "argument contains unquoted glob (`*`/`?`/`[`/`{`); shell expansion would diverge from what policy sees — quote it (e.g. '*.yaml') or refuse"
		}
		if i == 0 && strings.HasPrefix(lit.Value, "~") {
			return "argument starts with unquoted `~`; shell would tilde-expand it before the tool sees it — quote it or use the absolute path"
		}
		if hasTildeAfterAssignOrColon(lit.Value) {
			return "argument has unquoted `~` after `=` or `:` (e.g. `KEY=~/x`, `PATH=...:~/bin`); bash tilde-expands these before the tool sees them — quote the value"
		}
	}
	return ""
}

func hasTildeAfterAssignOrColon(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if (s[i] == '=' || s[i] == ':') && s[i+1] == '~' {
			return true
		}
	}
	return false
}

func flattenWord(w *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, p := range w.Parts {
		switch v := p.(type) {
		case *syntax.Lit:
			b.WriteString(v.Value)
		case *syntax.SglQuoted:
			b.WriteString(v.Value)
		case *syntax.DblQuoted:
			for _, qp := range v.Parts {
				if lit, ok := qp.(*syntax.Lit); ok {
					b.WriteString(lit.Value)
				} else {
					return "", false
				}
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

func isUnknownDashCWrapper(flat []string) bool {
	if len(flat) == 0 {
		return false
	}
	base := filepath.Base(flat[0])
	if dashCWrappers[base] {
		return false
	}
	if !knownShellNames[base] {
		return false
	}
	for i := 1; i < len(flat); i++ {
		if flat[i] == "-c" {
			return true
		}
	}
	return false
}

func isEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c == '_':
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func positionOf(s *syntax.Stmt) (int, int) {
	if s == nil || s.Position.Line() == 0 {
		return 0, 0
	}
	return int(s.Position.Line()), int(s.Position.Col())
}
