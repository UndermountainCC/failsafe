// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/UndermountainCC/failsafe/internal/auditlog"
	"github.com/UndermountainCC/failsafe/internal/config"
	"github.com/UndermountainCC/failsafe/internal/subcommand"
)

const version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Load config once at the top so all subcommands that need it can share it.
	// Fail-closed: a bad config.yaml must not allow the hook to run permissively.
	cfg, err := config.Load(config.Options{Home: os.Getenv("HOME"), Env: os.Getenv})
	if err != nil {
		fmt.Fprintf(stderr, "failsafe: load config: %v\n", err)
		return 1
	}

	if len(args) >= 2 {
		switch args[1] {
		case "--version", "-v":
			fmt.Fprintf(stdout, "failsafe %s\n", version)
			return 0
		case "--help", "-h":
			printHelp(stdout)
			return 0
		case "hook":
			return subcommand.Hook(stdin, stdout, stderr, subcommand.HookOptions{Cfg: cfg})
		case "mcp":
			return subcommand.MCP(stdin, stdout, stderr)
		case "explain":
			cwd, _ := os.Getwd()
			return subcommand.Explain(args[2:], stdout, subcommand.ExplainOptions{
				Home: os.Getenv("HOME"),
				CWD:  cwd,
			})
		case "test":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "usage: failsafe test <path>")
				return 2
			}
			return subcommand.TestCorpus(args[2], stdout, subcommand.TestOptions{Home: os.Getenv("HOME")})
		case "validate":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "usage: failsafe validate [--strict] <path>")
				return 2
			}
			fs := flag.NewFlagSet("validate", flag.ContinueOnError)
			fs.SetOutput(stderr)
			strict := fs.Bool("strict", false, "promote warnings to errors")
			if err := fs.Parse(args[2:]); err != nil {
				return 2
			}
			rest := fs.Args()
			if len(rest) < 1 {
				fmt.Fprintln(stderr, "usage: failsafe validate [--strict] <path>")
				return 2
			}
			return subcommand.Validate(rest[0], stdout, subcommand.ValidateOptions{Strict: *strict})
		case "trust":
			cwd, _ := os.Getwd()
			return subcommand.Trust(args[2:], stdout, subcommand.TrustOptions{
				Home: os.Getenv("HOME"),
				CWD:  cwd,
			})
		case "audit":
			cwd, _ := os.Getwd()
			if len(args) >= 3 {
				cwd = args[2]
			}
			return subcommand.Audit(cwd, stdout, subcommand.AuditOptions{Home: os.Getenv("HOME")})
		case "report":
			// Read from wherever the hook writes: reuse DefaultLogger's path
			// resolution so FAILSAFE_LOG and the default home path agree.
			home := os.Getenv("HOME")
			return subcommand.Report(args[2:], stdout, subcommand.ReportOptions{
				Home:    home,
				LogPath: auditlog.DefaultLogger(home, os.Getenv).Path,
			})
		case "log":
			home := os.Getenv("HOME")
			return subcommand.Log(args[2:], stdout, subcommand.LogOptions{
				Home:    home,
				LogPath: auditlog.DefaultLogger(home, os.Getenv).Path,
			})
		case "toggle":
			return subcommand.Toggle(stdout, subcommand.ToggleOptions{
				Chain: subcommand.DefaultModeChain(),
				Env:   subcommand.EnvFromOS(),
			})
		case "tools":
			if len(args) < 3 || args[2] != "list" {
				fmt.Fprintln(stderr, "usage: failsafe tools list")
				return 2
			}
			return subcommand.ToolsList(stdout, subcommand.ToolsListOptions{Home: os.Getenv("HOME")})
		case "policies":
			if len(args) < 3 || args[2] != "list" {
				fmt.Fprintln(stderr, "usage: failsafe policies list")
				return 2
			}
			cwd, _ := os.Getwd()
			return subcommand.PoliciesList(stdout, subcommand.PoliciesListOptions{
				Home: os.Getenv("HOME"),
				CWD:  cwd,
			})
		case "mode":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "usage: failsafe mode get | mode set <value>")
				return 2
			}
			switch args[2] {
			case "get":
				return subcommand.ModeGet(stdout, subcommand.ModeOptions{
					Chain: subcommand.DefaultModeChain(),
					Env:   subcommand.EnvFromOS(),
				})
			case "set":
				if len(args) < 4 {
					fmt.Fprintln(stderr, "usage: failsafe mode set <value>")
					return 2
				}
				return subcommand.ModeSet(args[3], stdout, subcommand.ModeOptions{
					Chain: subcommand.DefaultModeChain(),
					Env:   subcommand.EnvFromOS(),
				})
			default:
				fmt.Fprintf(stderr, "unknown mode action: %s\n", args[2])
				return 2
			}
		}
	}
	// No subcommand or unknown — default to hook mode (matches Claude Code's invocation).
	if hasNoFlagArgs(args[1:]) {
		return subcommand.Hook(stdin, stdout, stderr, subcommand.HookOptions{Cfg: cfg})
	}
	// Unknown invocation
	fmt.Fprintf(stderr, "failsafe: unknown invocation %v\n", args[1:])
	printHelp(stderr)
	return 2
}

func hasNoFlagArgs(args []string) bool {
	for _, a := range args {
		if len(a) > 0 && a[0] == '-' {
			return false
		}
	}
	return true
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `failsafe — command-policy hook for Claude Code

Usage:
  failsafe                       read hook JSON on stdin (default)
  failsafe hook                  same; explicit
  failsafe report [flags]        summarize the decision log
                                   [--since 7d] [--format md|json] [--share]
  failsafe log [flags]           inspect the raw decision log
                                   [--tail 20] [--since DUR] [--json]
  failsafe --version             print version
  failsafe --help                this help

Other subcommands: toggle, mode, tools list, policies list, explain,
test, validate, audit.`)
}
