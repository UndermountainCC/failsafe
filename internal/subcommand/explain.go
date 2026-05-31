// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/UndermountainCC/failsafe/internal/enrich"
	"github.com/UndermountainCC/failsafe/internal/facts"
	"github.com/UndermountainCC/failsafe/internal/policy"
	"github.com/UndermountainCC/failsafe/internal/shellparser"
	"github.com/UndermountainCC/failsafe/internal/trust"
)

type ExplainOptions struct {
	Home string
	CWD  string
	Mode string
	Now  time.Time
}

// Explain evaluates a literal shell command through the same pipeline as the
// hook subcommand and prints what would happen, per-call. Mirrors the
// hook's flow: shellparser.Extract → trust-aware policy chain → engine
// (per effective cwd) → decision per EffectiveCall. Stops on first block.
func Explain(cmdArgs []string, out io.Writer, opts ExplainOptions) int {
	// cmdArgs arrives shell-split by the user's shell.
	//   1. Multiple args ⇒ the user passed argv directly. Reassemble with
	//      shell-safe quoting so shellparser sees an equivalent input.
	//   2. Single arg containing whitespace ⇒ the user passed the whole
	//      command as one literal; feed it to the parser as-is.
	var command string
	if len(cmdArgs) == 1 && strings.ContainsAny(cmdArgs[0], " \t") {
		command = cmdArgs[0]
	} else {
		command = joinShellArgs(cmdArgs)
	}

	calls, refuse, err := shellparser.Extract(command)
	if err != nil {
		fmt.Fprintf(out, "shell parse error: %v\n", err)
		return 2
	}
	if refuse != "" {
		fmt.Fprintf(out, "Decision: BLOCK (refuse-on-ambiguity)\n")
		fmt.Fprintf(out, "Reason  : %s\n", refuse)
		return 0
	}
	if len(calls) == 0 {
		fmt.Fprintf(out, "no calls extracted from: %s\n", command)
		return 0
	}

	tr, err := trust.Load(opts.Home)
	if err != nil {
		fmt.Fprintf(out, "trust load: %v\n", err)
		return 2
	}
	reg, err := buildRegistry(opts.Home)
	if err != nil {
		fmt.Fprintf(out, "registry build: %v\n", err)
		return 2
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	mode := opts.Mode
	if mode == "" {
		mode = "read"
	}

	engineByCwd := map[string]*policy.Engine{}
	getEngine := func(cwd string) (*policy.Engine, []policy.Module, error) {
		if e, ok := engineByCwd[cwd]; ok {
			// Discover runs again only for display (cheap); cache the engine.
			mods, _ := policy.Discover(policy.DiscoverOpts{
				BundledLoader: loadBundledPolicies,
				Home:          opts.Home,
				CWD:           cwd,
				IsTrusted:     tr.IsTrusted,
			})
			return e, mods, nil
		}
		mods, err := policy.Discover(policy.DiscoverOpts{
			BundledLoader: loadBundledPolicies,
			Home:          opts.Home,
			CWD:           cwd,
			IsTrusted:     tr.IsTrusted,
		})
		if err != nil {
			return nil, nil, err
		}
		eng, err := policy.New(cwd, mods)
		if err != nil {
			return nil, nil, err
		}
		engineByCwd[cwd] = eng
		return eng, mods, nil
	}

	for i, call := range calls {
		tool, ok := reg.Find(call)
		if !ok {
			fmt.Fprintf(out, "[call %d] %s — not an infra tool, skipped\n", i+1, call.Name)
			continue
		}
		if call.UncertainCwd {
			fmt.Fprintf(out, "── call %d: %s ──\n", i+1, tool.Name())
			fmt.Fprintln(out, "Decision: BLOCK (uncertain cwd)")
			fmt.Fprintln(out, "Reason  : ambiguous cd preceded a registered tool call (use `cd PATH && CMD` instead of cd-then-semicolon or cd-then-||)")
			fmt.Fprintln(out)
			return 0
		}
		parsed, err := tool.Parse(call.Args)
		if err != nil {
			fmt.Fprintf(out, "[call %d] parse error in %s: %v\n", i+1, tool.Name(), err)
			return 2
		}
		effectiveCWD := opts.CWD
		if call.Cwd != "" {
			if filepath.IsAbs(call.Cwd) {
				effectiveCWD = call.Cwd
			} else {
				effectiveCWD = filepath.Clean(filepath.Join(opts.CWD, call.Cwd))
			}
		}
		engine, mods, err := getEngine(effectiveCWD)
		if err != nil {
			fmt.Fprintf(out, "[call %d] %v\n", i+1, err)
			return 2
		}
		enrichReg := enrich.NewRegistry()
		enrichReg.Register(enrich.GitEnricher{CWD: effectiveCWD})
		enrichReg.Register(enrich.KubectlContextEnricher{})
		fact := facts.Builder{
			Mode: mode, Tool: tool.Name(), CWD: effectiveCWD, Now: now,
			Parsed: parsed, Env: call.Env,
			EnricherNames: tool.Enrichers(), Enrichers: enrichReg,
			Raw: command,
		}.Build()
		dec, err := engine.Evaluate(context.Background(), fact)
		if err != nil {
			fmt.Fprintf(out, "[call %d] eval: %v\n", i+1, err)
			return 2
		}

		fmt.Fprintf(out, "── call %d: %s ──\n", i+1, tool.Name())
		fmt.Fprintf(out, "Verb:        %s\n", parsed.Verb)
		if parsed.Subverb != "" {
			fmt.Fprintf(out, "Subverb:     %s\n", parsed.Subverb)
		}
		if len(parsed.Flags) > 0 {
			fmt.Fprintln(out, "Flags:")
			for k, v := range parsed.Flags {
				fmt.Fprintf(out, "  %-15s = %v\n", k, v)
			}
		}
		if len(parsed.Positional) > 0 {
			fmt.Fprintf(out, "Positional:  %s\n", strings.Join(parsed.Positional, " "))
		}
		fmt.Fprintf(out, "Effective cwd: %s\n", effectiveCWD)
		fmt.Fprintf(out, "Mode:        %s\n", mode)
		fmt.Fprintf(out, "Policy chain (%d modules at this cwd):\n", len(mods))
		for _, m := range mods {
			tag := ""
			if m.Layer == policy.LayerRepo {
				if m.Trusted {
					tag = " [trusted]"
				} else {
					tag = " [untrusted]"
				}
			}
			fmt.Fprintf(out, "  [%s] %s%s\n", m.Layer, m.File, tag)
		}
		if dec.Block {
			fmt.Fprintln(out, "Decision: BLOCK")
			fmt.Fprintf(out, "Reason  : %s\n", dec.Reason)
			return 0 // first block stops the explanation, mirroring the hook
		}
		if dec.OverrideReason != "" {
			fmt.Fprintln(out, "Decision: ALLOW (with override)")
			fmt.Fprintf(out, "Override: %s\n", dec.OverrideReason)
		} else {
			fmt.Fprintln(out, "Decision: ALLOW")
		}
		fmt.Fprintln(out)
	}
	return 0
}

// joinShellArgs reassembles argv into a quote-safe shell command string.
// shellparser then re-parses it the same way it would parse a real shell input.
func joinShellArgs(argv []string) string {
	var b strings.Builder
	for i, a := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		if strings.ContainsAny(a, " \t\"'\\;&|<>()*?$`#~!\n") {
			// double-quote it; escape inner double-quotes and backslashes
			b.WriteByte('"')
			for _, r := range a {
				if r == '"' || r == '\\' {
					b.WriteByte('\\')
				}
				b.WriteRune(r)
			}
			b.WriteByte('"')
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}
