// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package subcommand

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/UndermountainCC/failsafe/internal/policy"
)

type TestOptions struct {
	Home string
}

type expected struct {
	Block            bool   `json:"block"`
	ReasonContains   string `json:"reason_contains"`
	OverrideContains string `json:"override_reason_contains"`
}

// TestCorpus runs every (fact.json, expected.json) pair under path. If path is
// a directory, walks it. If path is a single fact.json, runs only that case.
//
// The engine is built per-case using the fact's `cwd` field, because the
// provenance-aware engine (Task 19) sorts reasons by closest-to-cwd. A corpus
// case that depends on closest-cwd ordering must therefore set fact.cwd to a
// realistic value; cases that don't care can leave it empty (engine treats
// empty cwd as "anything beats LayerBundled, ties broken by layer rank").
func TestCorpus(path string, out io.Writer, opts TestOptions) int {
	cases, err := collectCases(path)
	if err != nil {
		fmt.Fprintf(out, "%v\n", err)
		return 2
	}
	pass, fail := 0, 0
	for _, c := range cases {
		ok, msg := runCase(opts, c)
		status := "PASS"
		if !ok {
			status = "FAIL"
			fail++
		} else {
			pass++
		}
		fmt.Fprintf(out, "%s: %s%s\n", status, c.name, msg)
	}
	fmt.Fprintf(out, "\n%d passed, %d failed.\n", pass, fail)
	if fail > 0 {
		return 1
	}
	return 0
}

type corpusCase struct {
	name     string
	factPath string
	expPath  string
}

func collectCases(path string) ([]corpusCase, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		// single fact.json
		dir := filepath.Dir(path)
		return []corpusCase{{
			name:     filepath.Base(dir),
			factPath: path,
			expPath:  filepath.Join(dir, "expected.json"),
		}}, nil
	}
	var out []corpusCase
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		fact := filepath.Join(path, e.Name(), "fact.json")
		exp := filepath.Join(path, e.Name(), "expected.json")
		if _, err := os.Stat(fact); err != nil {
			continue
		}
		out = append(out, corpusCase{name: e.Name(), factPath: fact, expPath: exp})
	}
	return out, nil
}

func runCase(opts TestOptions, c corpusCase) (bool, string) {
	factBody, err := os.ReadFile(c.factPath)
	if err != nil {
		return false, ": " + err.Error()
	}
	expBody, err := os.ReadFile(c.expPath)
	if err != nil {
		return false, ": " + err.Error()
	}
	var fact map[string]any
	if err := json.Unmarshal(factBody, &fact); err != nil {
		return false, ": fact.json: " + err.Error()
	}
	var exp expected
	if err := json.Unmarshal(expBody, &exp); err != nil {
		return false, ": expected.json: " + err.Error()
	}

	// The engine is provenance-aware (closest-to-cwd reason precedence). Build
	// it with the fact's own cwd so multi-layer cases produce stable output.
	cwd, _ := fact["cwd"].(string)
	mods, err := policy.Discover(policy.DiscoverOpts{
		BundledLoader: loadBundledPolicies,
		Home:          opts.Home,
		CWD:           cwd,
	})
	if err != nil {
		return false, ": discover: " + err.Error()
	}
	engine, err := policy.New(cwd, mods)
	if err != nil {
		return false, ": compile: " + err.Error()
	}

	dec, err := engine.Evaluate(context.Background(), fact)
	if err != nil {
		return false, ": eval: " + err.Error()
	}
	if dec.Block != exp.Block {
		return false, fmt.Sprintf(": block=%v, expected %v (reason=%q)", dec.Block, exp.Block, dec.Reason)
	}
	if exp.ReasonContains != "" && !strings.Contains(dec.Reason, exp.ReasonContains) {
		return false, fmt.Sprintf(": reason %q does not contain %q", dec.Reason, exp.ReasonContains)
	}
	if exp.OverrideContains != "" && !strings.Contains(dec.OverrideReason, exp.OverrideContains) {
		return false, fmt.Sprintf(": override %q does not contain %q", dec.OverrideReason, exp.OverrideContains)
	}
	return true, ""
}
