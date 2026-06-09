// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package subcommand contains one file per CLI subcommand. Each subcommand
// is a pure function from (stdin, stdout, stderr, options) to int (exit code),
// matching cmd/failsafe/main.go's run() shape so subcommands are testable
// without os.Exit.
package subcommand

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UndermountainCC/failsafe/internal/auditlog"
	"github.com/UndermountainCC/failsafe/internal/config"
	embedfs "github.com/UndermountainCC/failsafe/internal/embed"
	"github.com/UndermountainCC/failsafe/internal/enrich"
	"github.com/UndermountainCC/failsafe/internal/facts"
	"github.com/UndermountainCC/failsafe/internal/hookio"
	"github.com/UndermountainCC/failsafe/internal/mode"
	"github.com/UndermountainCC/failsafe/internal/policy"
	"github.com/UndermountainCC/failsafe/internal/shellparser"
	"github.com/UndermountainCC/failsafe/internal/tools"
	"github.com/UndermountainCC/failsafe/internal/trust"
)

// HookOptions overrides defaults for testing.
type HookOptions struct {
	Home         string
	ModeOverride string
	ModeChain    *mode.Chain
	Now          time.Time
	BundledLoad  func() ([]policy.Module, error)
	// Logger receives one record per infra-tool decision (and per
	// refuse/parse block). Nil → resolved from cfg.Log (or home + FAILSAFE_LOG
	// when Cfg is nil). Logging is best-effort and never fails the hook.
	Logger *auditlog.Logger
	// Cfg is the loaded configuration. Nil → defaults are loaded via
	// config.Load so there is a single code path regardless of caller.
	// When non-nil, Cfg drives the mode chain, audit logger, policy paths,
	// tools dir, and trust path instead of hardcoded literals.
	Cfg *config.Config
}

// Hook is the default subcommand: read Claude Code hook JSON, decide, emit.
func Hook(stdin io.Reader, stdout, stderr io.Writer, opts HookOptions) int {
	in, err := hookio.Read(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "failsafe: read hook input: %v\n", err)
		return 1
	}
	if in.ToolInput.Command == "" {
		return 0
	}

	home := opts.Home
	if home == "" {
		home = os.Getenv("HOME")
	}

	// Resolve config: use caller-supplied Cfg or load defaults (no config file
	// → all defaults = today's hardcoded values; fail-closed on a bad file).
	cfg := opts.Cfg
	if cfg == nil {
		loaded, err := config.Load(config.Options{Home: home, Env: os.Getenv})
		if err != nil {
			fmt.Fprintf(stderr, "failsafe: load config: %v\n", err)
			return 1
		}
		cfg = loaded
	}

	// 1. Resolve mode.
	modeVal := opts.ModeOverride
	if modeVal == "" {
		chain := opts.ModeChain
		if chain == nil {
			chain = buildModeChain(cfg, home)
		}
		val, _, err := chain.Resolve(envWithHome(home))
		if err != nil {
			fmt.Fprintf(stderr, "failsafe: mode resolution: %v\n", err)
			return 1
		}
		modeVal = val
	}

	// Decision logger: best-effort JSON-Lines trail. Resolved here (after
	// mode, so modeVal is in scope) and used at every decision point below.
	// Logging never fails the hook — logRec ignores Log's error.
	lg := opts.Logger
	if lg == nil {
		lg = loggerFromConfig(cfg)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	pane := os.Getenv("WEZTERM_PANE")
	logRec := func(decision, reason, tool, verb, subverb, cwd string) {
		_ = lg.Log(auditlog.Record{
			Time: now, Decision: decision, Reason: reason, Mode: modeVal,
			Tool: tool, Verb: verb, Subverb: subverb, CWD: cwd,
			Command: in.ToolInput.Command, AgentType: "claude-code",
			SessionID: in.SessionID, Pane: pane,
		})
	}

	// 2. Parse the command into EffectiveCalls. Inability-to-inspect IS an
	//    authorization decision per spec §3.5: parse errors AND refuses
	//    both block. (A previous version allowed parse errors on the theory
	//    that a malformed command "wouldn't run anyway" — but mvdan parses
	//    a strict subset of valid shell, so an obscure-but-valid command
	//    might fail our parser yet still execute. Failing closed is
	//    correct for a guard.)
	calls, refusal, err := shellparser.Extract(in.ToolInput.Command)
	if err != nil {
		reason := "failsafe cannot parse this command (shell syntax not supported by mvdan.cc/sh): " + err.Error()
		logRec("block", reason, "shell", "unanalyzable", "parse", in.CWD)
		_ = hookio.WriteBlock(stdout, reason)
		return 0
	}
	if refusal != nil {
		reason := "failsafe cannot safely analyze this command: " + refusal.Message
		logRec("block", reason, "shell", "unanalyzable", refusal.Kind, in.CWD)
		_ = hookio.WriteBlock(stdout, reason)
		return 0
	}

	// 3. Set up shared inputs: trust file, bundled-policy loader, registry.
	tr, err := trust.LoadFromPath(cfg.Trust.Path)
	if err != nil {
		fmt.Fprintf(stderr, "failsafe: trust load: %v\n", err)
		return 1
	}
	bundledLoad := opts.BundledLoad
	if bundledLoad == nil {
		bundledLoad = loadBundledPolicies
	}
	reg, err := buildRegistry(cfg.Policy.ToolsDir)
	if err != nil {
		// Fail-closed: a corrupt bundled tool YAML or a malformed user
		// tool YAML (which would silently bypass policy for that tool's
		// commands) must not start the hook. Emit a block so the user
		// sees the error instead of unexpected allows.
		_ = hookio.WriteBlock(stdout, "failsafe cannot start: "+err.Error())
		return 0
	}

	// 4. Per-call evaluation. The policy chain depends on the EFFECTIVE
	//    cwd (which can shift via `cd dir && ...`), so we cache engines
	//    by effective cwd. Most commands have one effective cwd; the
	//    cache keeps the common path single-build.
	engineByCwd := map[string]*policy.Engine{}
	getEngine := func(cwd string) (*policy.Engine, error) {
		if e, ok := engineByCwd[cwd]; ok {
			return e, nil
		}
		mods, err := policy.Discover(policy.DiscoverOpts{
			BundledLoader:  bundledLoad,
			Home:           home,
			CWD:            cwd,
			UserPolicyPath: cfg.Policy.UserPath,
			IsTrusted:      tr.IsTrusted,
		})
		if err != nil {
			return nil, fmt.Errorf("policy discovery: %w", err)
		}
		eng, err := policy.New(cwd, mods)
		if err != nil {
			return nil, fmt.Errorf("policy compile: %w", err)
		}
		engineByCwd[cwd] = eng
		return eng, nil
	}

	var finalOverride string
	for _, call := range calls {
		tool, ok := reg.Find(call)
		if !ok {
			continue // not an infra tool we know about
		}
		// Registered tool with uncertain cwd would evaluate against a
		// potentially wrong cwd (the walker tracked one path, but at runtime
		// the cd may have failed). Refuse rather than risk a policy mismatch.
		// Non-registered calls (echo, ls, gh, etc.) above were skipped before
		// reaching this gate — they're cwd-insensitive enough to allow.
		if call.UncertainCwd {
			reason := "failsafe cannot safely analyze this command: ambiguous cd preceded a registered tool call (use `cd PATH && CMD` instead of cd-then-semicolon or cd-then-||)"
			logRec("block", reason, tool.Name(), "", "", in.CWD)
			_ = hookio.WriteBlock(stdout, reason)
			return 0
		}
		parsed, err := tool.Parse(call.Args)
		if err != nil {
			fmt.Fprintf(stderr, "failsafe: parse error in %s: %v\n", tool.Name(), err)
			return 1
		}
		// Effective cwd for this call: a leading `cd PATH && ...` populates
		// call.Cwd (spec §3.5). Relative paths resolve against in.CWD;
		// absolute replaces; empty falls back to in.CWD.
		effectiveCWD := in.CWD
		if call.Cwd != "" {
			if filepath.IsAbs(call.Cwd) {
				effectiveCWD = call.Cwd
			} else {
				effectiveCWD = filepath.Clean(filepath.Join(in.CWD, call.Cwd))
			}
		}

		engine, err := getEngine(effectiveCWD)
		if err != nil {
			fmt.Fprintf(stderr, "failsafe: %v\n", err)
			return 1
		}

		fact := facts.Builder{
			Mode:          modeVal,
			Tool:          tool.Name(),
			CWD:           effectiveCWD, // git enricher + repo discovery use the same cwd
			Now:           now,
			SessionID:     in.SessionID,
			Pane:          os.Getenv("WEZTERM_PANE"),
			Parsed:        parsed,
			Env:           call.Env,
			Raw:           in.ToolInput.Command,
			EnricherNames: tool.Enrichers(),
			Enrichers:     buildEnricherRegistry(effectiveCWD),
		}.Build()

		dec, err := engine.Evaluate(context.Background(), fact)
		if err != nil {
			fmt.Fprintf(stderr, "failsafe: policy eval: %v\n", err)
			return 1
		}
		if dec.Block {
			logRec("block", dec.Reason, tool.Name(), parsed.Verb, parsed.Subverb, effectiveCWD)
			_ = hookio.WriteBlock(stdout, dec.Reason)
			return 0
		}
		// Allowed (with or without a repo override). Log per registered-tool
		// call so the trail records each infra action at its true granularity.
		if dec.OverrideReason != "" {
			logRec("allow_override", dec.OverrideReason, tool.Name(), parsed.Verb, parsed.Subverb, effectiveCWD)
			if finalOverride == "" {
				finalOverride = dec.OverrideReason
			}
		} else {
			logRec("allow", "", tool.Name(), parsed.Verb, parsed.Subverb, effectiveCWD)
		}
	}

	if finalOverride != "" {
		_ = hookio.WriteAllowWithOverride(stdout, finalOverride)
	}
	return 0
}

// DefaultModeChain returns the same mode chain used by the hook subcommand.
// Exposed so the toggle and mode subcommands share the chain definition.
// It uses config defaults (identical to the previous hardcoded values).
func DefaultModeChain() *mode.Chain { return defaultModeChain() }

// EnvFromOS returns os.Environ() as a map.
func EnvFromOS() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}

// defaultModeChain returns the chain built from compile-time defaults.
// Kept for DefaultModeChain() and the toggle/mode subcommands that call it
// without a config.
func defaultModeChain() *mode.Chain {
	cfg, _ := config.Load(config.Options{Home: os.Getenv("HOME"), Env: os.Getenv})
	if cfg == nil {
		// Absolute fallback: should never happen (Load only fails on a bad
		// config file, and an absent file is fine). Return the hardcoded chain.
		return &mode.Chain{
			Sources: []mode.Source{
				mode.EnvSource{Name: "FAILSAFE_MODE"},
				mode.FileSource{Pattern: "${HOME}/.claude/pane-mode/${WEZTERM_PANE}"},
				mode.FileSource{Pattern: "${HOME}/.claude/pane-mode/${TMUX_PANE}"},
				mode.FileSource{Pattern: "${HOME}/.claude/pane-mode/${ITERM_SESSION_ID}"},
				mode.FileSource{Pattern: "${HOME}/.claude/pane-mode/${KITTY_WINDOW_ID}"},
				mode.FileSource{Pattern: "${HOME}/.claude/pane-mode/${CLAUDE_SESSION_ID}"},
				mode.TTYSource{Dir: "${HOME}/.config/failsafe"},
				mode.FileSource{Pattern: "${HOME}/.config/failsafe/mode"},
			},
			Default: "enabled",
		}
	}
	return buildModeChain(cfg, os.Getenv("HOME"))
}

// buildModeChain builds the mode.Chain from a loaded *config.Config.
// home is the resolved home directory (already expanded, no tilde).
// The source ORDER and kinds are fixed in code; only cfg.Mode.PaneDir and
// cfg.Mode.Default are driven by config (spec §5 "recorded not driven").
func buildModeChain(cfg *config.Config, home string) *mode.Chain {
	paneDir := cfg.Mode.PaneDir
	// If PaneDir is already an absolute path (expanded by config.Load), use it
	// directly; otherwise fall back to the ${HOME}-prefixed pattern form so the
	// mode chain's own ${VAR} expansion still works.
	paneDirPattern := func(varName string) string {
		// config.Load expands tildes; paneDir is already absolute.
		// We still want ${WEZTERM_PANE} etc. resolved by the chain at
		// resolve-time, so append the variable placeholder.
		return paneDir + "/${" + varName + "}"
	}
	return &mode.Chain{
		Sources: []mode.Source{
			mode.EnvSource{Name: "FAILSAFE_MODE"},
			mode.FileSource{Pattern: paneDirPattern("WEZTERM_PANE")},
			mode.FileSource{Pattern: paneDirPattern("TMUX_PANE")},
			mode.FileSource{Pattern: paneDirPattern("ITERM_SESSION_ID")},
			mode.FileSource{Pattern: paneDirPattern("KITTY_WINDOW_ID")},
			mode.FileSource{Pattern: paneDirPattern("CLAUDE_SESSION_ID")},
			// Per-controlling-tty: gives a plain shell (no multiplexer var) its
			// own writable mode instead of sharing the single global file.
			mode.TTYSource{Dir: "${HOME}/.config/failsafe"},
			// Global last-resort fallback (always writable while HOME is set).
			mode.FileSource{Pattern: "${HOME}/.config/failsafe/mode"},
		},
		Default: cfg.Mode.Default,
	}
}

// loggerFromConfig builds an *auditlog.Logger from cfg.Log.
// Mirrors the semantics of auditlog.DefaultLogger:
//   - Disabled → empty Logger (no-op)
//   - Enabled with path → file logger at that path
func loggerFromConfig(cfg *config.Config) *auditlog.Logger {
	if !cfg.Log.Enabled {
		return &auditlog.Logger{}
	}
	return &auditlog.Logger{Path: cfg.Log.Path}
}

func envWithHome(home string) map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	out["HOME"] = home
	return out
}

// buildRegistry assembles the tool registry from Go-coded tools (kubectl,
// helm), bundled YAML tools, and user-provided YAML tools. toolsDir is the
// fully-expanded path to the user tools directory (e.g. from cfg.Policy.ToolsDir).
// Errors are fail-closed: a corrupt bundled YAML means the binary is broken,
// and a malformed user YAML means commands for that tool would silently bypass
// policy. Both must surface to the caller, not be silently skipped.
func buildRegistry(toolsDir string) (*tools.Registry, error) {
	r := tools.NewRegistry()
	r.Add(tools.NewKubectl())
	r.Add(tools.NewHelm())
	r.Add(tools.NewFailsafeTool())
	for _, name := range embedfs.BundledToolNames() {
		body, err := embedfs.ReadBundledTool(name)
		if err != nil {
			return nil, fmt.Errorf("read bundled tool %s: %w", name, err)
		}
		t, err := tools.LoadYAMLTool(strings.NewReader(string(body)))
		if err != nil {
			return nil, fmt.Errorf("parse bundled tool %s: %w", name, err)
		}
		r.Add(t)
	}
	if toolsDir != "" {
		userTools := os.DirFS(toolsDir)
		entries, err := fs.ReadDir(userTools, ".")
		// fs.ReadDir on a nonexistent dir returns an error — that's the
		// common case (no user tools); tolerate it. Other errors propagate.
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read user tools dir %s: %w", toolsDir, err)
			}
			return r, nil
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			body, err := fs.ReadFile(userTools, e.Name())
			if err != nil {
				return nil, fmt.Errorf("read user tool %s: %w", e.Name(), err)
			}
			t, err := tools.LoadYAMLTool(strings.NewReader(string(body)))
			if err != nil {
				return nil, fmt.Errorf("parse user tool %s: %w", e.Name(), err)
			}
			r.Add(t)
		}
	}
	return r, nil
}

// buildEnricherRegistry returns a populated enrich.Registry for this call's
// effective cwd. Returning the registry (not a flat slice) lets the fact
// builder invoke enrichers via Registry.RunAll, which enforces the §3.6
// contract: 100ms timeout per enricher and recover() on panic.
//
// Per-call construction is intentional — the GitEnricher closes over `cwd`,
// which can shift across calls due to `cd dir && ...`. A registry built once
// outside the loop would bake the wrong cwd into the git enricher.
func buildEnricherRegistry(cwd string) *enrich.Registry {
	reg := enrich.NewRegistry()
	reg.Register(enrich.GitEnricher{CWD: cwd})
	reg.Register(enrich.KubectlContextEnricher{})
	return reg
}

func loadBundledPolicies() ([]policy.Module, error) {
	var out []policy.Module
	for _, name := range embedfs.BundledPolicyNames() {
		body, err := embedfs.ReadBundledPolicy(name)
		if err != nil {
			return nil, err
		}
		out = append(out, policy.Module{
			Layer: policy.LayerBundled,
			File:  "bundled/" + name,
			Body:  string(body),
		})
	}
	return out, nil
}
