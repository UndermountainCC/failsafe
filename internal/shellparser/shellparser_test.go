// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package shellparser

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtract_Plain(t *testing.T) {
	calls, refuse, err := Extract("kubectl get pods")
	if err != nil {
		t.Fatal(err)
	}
	if refuse != "" {
		t.Errorf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("calls = %+v", calls)
	}
	if !reflect.DeepEqual(calls[0].Args, []string{"get", "pods"}) {
		t.Errorf("args = %v", calls[0].Args)
	}
}

func TestExtract_FlagBeforeVerbQuoted(t *testing.T) {
	calls, _, err := Extract(`kubectl --context "arn with spaces" get pods`)
	if err != nil {
		t.Fatal(err)
	}
	if calls[0].Args[1] != "arn with spaces" {
		t.Errorf("quoted arg lost: %v", calls[0].Args)
	}
}

func TestExtract_EnvPrefix(t *testing.T) {
	calls, _, _ := Extract("KUBECONFIG=/tmp/x kubectl get pods")
	if calls[0].Name != "kubectl" {
		t.Errorf("call name = %q, want kubectl", calls[0].Name)
	}
	if calls[0].Env["KUBECONFIG"] != "/tmp/x" {
		t.Errorf("env = %v", calls[0].Env)
	}
}

func TestExtract_ChainedCommands(t *testing.T) {
	calls, _, _ := Extract("echo ok && kubectl apply -f x.yaml")
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %+v", len(calls), calls)
	}
	if calls[0].Name != "echo" || calls[1].Name != "kubectl" {
		t.Errorf("calls = %+v", calls)
	}
}

func TestExtract_Pipe(t *testing.T) {
	calls, _, _ := Extract("echo yes | kubectl apply -f -")
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (both gated), got %+v", calls)
	}
}

func TestExtract_UnwrapShDashC(t *testing.T) {
	calls, refuse, _ := Extract(`sh -c "kubectl apply -f x.yaml"`)
	if refuse != "" {
		t.Errorf("should unwrap sh -c, got refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("expected unwrapped kubectl call, got %+v", calls)
	}
}

func TestExtract_RefuseBashLc(t *testing.T) {
	_, refuse, _ := Extract(`bash -lc 'kubectl --context arn get pods'`)
	if refuse == "" {
		t.Errorf("expected refuse on bash -lc form")
	}
}

func TestExtract_UnwrapEnv(t *testing.T) {
	calls, _, _ := Extract(`env FOO=bar kubectl get pods`)
	if calls[0].Name != "kubectl" {
		t.Errorf("call name = %q", calls[0].Name)
	}
	if calls[0].Env["FOO"] != "bar" {
		t.Errorf("env = %v", calls[0].Env)
	}
}

func TestExtract_RefuseSubshell(t *testing.T) {
	_, refuse, _ := Extract("(kubectl apply -f x.yaml)")
	if !strings.Contains(refuse, "subshell") {
		t.Errorf("expected subshell refuse, got %q", refuse)
	}
}

func TestExtract_CommandSubstNonHeadEmitsPlaceholder(t *testing.T) {
	// Pre-relax: walker refused on any command substitution. Post-relax:
	// walker emits a placeholder for non-head dynamic args; the bundled
	// policy then blocks `kubectl apply -f <dynamic>` because verb=apply
	// is not in read_verbs and allowed_dry_run requires a literal flag.
	calls, refuse, _ := Extract(`kubectl apply -f $(echo x.yaml)`)
	if refuse != "" {
		t.Fatalf("expected no walker refuse for non-head $(...); got: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Fatalf("expected one kubectl call, got %+v", calls)
	}
	// Last arg should be the placeholder.
	last := calls[0].Args[len(calls[0].Args)-1]
	if last != DynamicMarker {
		t.Errorf("last arg = %q, want %q", last, DynamicMarker)
	}
}

func TestExtract_DynamicHeadStillRefuses(t *testing.T) {
	_, refuse, _ := Extract(`$(echo kubectl) apply`)
	if refuse == "" {
		t.Errorf("expected refuse for dynamic head — head must be statically known")
	}
}

func TestExtract_GhPRCreateAcceptsDynamicBody(t *testing.T) {
	// The real-world unblocking case: `gh pr create --body "$(cat ...)"` is
	// the standard PR-creation pattern. gh isn't an infra tool, so the
	// registry skips it and the command runs.
	calls, refuse, _ := Extract(`gh pr create --title "X" --body "$(cat body.md)"`)
	if refuse != "" {
		t.Fatalf("expected no refuse for gh + dynamic body; got: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "gh" {
		t.Fatalf("expected one gh call, got %+v", calls)
	}
	found := false
	for _, a := range calls[0].Args {
		if a == DynamicMarker {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected DynamicMarker placeholder in args, got %v", calls[0].Args)
	}
}

func TestExtract_DynamicVerbEmitsPlaceholder(t *testing.T) {
	// `kubectl $(echo apply)` — head is static (kubectl), but the verb
	// position is dynamic. Walker emits with placeholder; bundled policy
	// blocks because the placeholder is not in read_verbs.
	calls, refuse, _ := Extract(`kubectl $(echo apply)`)
	if refuse != "" {
		t.Fatalf("expected no walker refuse; got: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Fatalf("expected kubectl call, got %+v", calls)
	}
	if len(calls[0].Args) != 1 || calls[0].Args[0] != DynamicMarker {
		t.Errorf("args = %v, want [%q]", calls[0].Args, DynamicMarker)
	}
}

func TestExtract_RefuseEval(t *testing.T) {
	_, refuse, _ := Extract(`eval "kubectl apply"`)
	if refuse == "" {
		t.Errorf("expected refuse for eval")
	}
}

func TestExtract_RefuseUnknownWrapperDashC(t *testing.T) {
	_, refuse, _ := Extract(`zsh -c "kubectl apply"`)
	if refuse == "" {
		t.Errorf("expected refuse for unknown wrapper -c form")
	}
	for _, cmd := range []string{
		`fish -c "kubectl apply"`,
		`ksh -c "kubectl apply"`,
		`/usr/local/bin/zsh -c "kubectl apply"`,
	} {
		_, r, _ := Extract(cmd)
		if r == "" {
			t.Errorf("expected refuse for %q", cmd)
		}
	}
}

func TestExtract_NonShellDashCNotRefused(t *testing.T) {
	for _, cmd := range []string{
		`tar -c -f x.tgz file1 file2`,
		`tar -czf x.tgz dir`,
		`cc -c file.c`,
		`gcc -c -o file.o file.c`,
	} {
		calls, refuse, _ := Extract(cmd)
		if refuse != "" {
			t.Errorf("did not expect refuse for %q; got: %s", cmd, refuse)
		}
		if len(calls) == 0 {
			t.Errorf("expected at least one call for %q", cmd)
		}
	}
}

func TestExtract_MultiCDChain(t *testing.T) {
	calls, refuse, _ := Extract("cd /abs && cd ./rel && kubectl get pods")
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Cwd != "/abs/rel" {
		t.Errorf("expected one call with Cwd=/abs/rel, got %+v", calls)
	}
}

func TestExtract_CDPropagatesCwd(t *testing.T) {
	calls, refuse, _ := Extract("cd /tmp/x && kubectl get pods")
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Fatalf("expected one kubectl call, got %+v", calls)
	}
	if calls[0].Cwd != "/tmp/x" {
		t.Errorf("cd-tracked Cwd = %q, want /tmp/x", calls[0].Cwd)
	}
}

func TestExtract_CDSemicolonEmitsUncertainCwd(t *testing.T) {
	calls, refuse, _ := Extract("cd /tmp/y; kubectl get pods")
	if refuse != "" {
		t.Fatalf("unexpected walker refuse: %s", refuse)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (kubectl), got %+v", calls)
	}
	if !calls[0].UncertainCwd {
		t.Errorf("expected UncertainCwd=true after cd-then-semicolon; got %+v", calls[0])
	}
}

func TestExtract_CDOrOpEmitsUncertainCwd(t *testing.T) {
	calls, refuse, _ := Extract("cd /missing || kubectl get pods")
	if refuse != "" {
		t.Fatalf("unexpected walker refuse: %s", refuse)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (kubectl), got %+v", calls)
	}
	if !calls[0].UncertainCwd {
		t.Errorf("expected UncertainCwd=true after cd-then-||; got %+v", calls[0])
	}
}

func TestExtract_CDStandaloneOK(t *testing.T) {
	_, refuse, _ := Extract("cd /tmp")
	if refuse != "" {
		t.Errorf("unexpected refuse on standalone cd: %s", refuse)
	}
}

func TestExtract_CallBeforeCDIsFine(t *testing.T) {
	calls, refuse, _ := Extract("kubectl get && cd /tmp")
	if refuse != "" {
		t.Errorf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("expected one kubectl call, got %+v", calls)
	}
}

func TestExtract_CDInANDLeakingPastBoundaryEmitsUncertainCwd(t *testing.T) {
	for _, cmd := range []string{
		`cd /tmp/nonexistent && true; kubectl apply -f x.yaml`,
		`cd /repo && do-something; kubectl apply`,
		`cd /a && cd /b; kubectl get`,
	} {
		calls, refuse, _ := Extract(cmd)
		if refuse != "" {
			t.Errorf("unexpected walker refuse for %q: %s", cmd, refuse)
			continue
		}
		// The trailing call (kubectl) must be emitted with UncertainCwd=true
		// — that's what tells the hook layer to refuse for registered tools
		// while still allowing cwd-insensitive trailing calls (echo, ls, gh).
		var trailing *EffectiveCall
		for i := range calls {
			if calls[i].Name == "kubectl" {
				trailing = &calls[i]
			}
		}
		if trailing == nil {
			t.Errorf("expected a kubectl call for %q; got %+v", cmd, calls)
			continue
		}
		if !trailing.UncertainCwd {
			t.Errorf("expected UncertainCwd=true on trailing kubectl for %q; got %+v", cmd, *trailing)
		}
	}
}

func TestExtract_CDInBashDashCLeakingPastBoundaryEmitsUncertainCwd(t *testing.T) {
	calls, refuse, _ := Extract(`bash -c 'cd /missing && true; kubectl apply -f x.yaml'`)
	if refuse != "" {
		t.Fatalf("unexpected walker refuse: %s", refuse)
	}
	var trailing *EffectiveCall
	for i := range calls {
		if calls[i].Name == "kubectl" {
			trailing = &calls[i]
		}
	}
	if trailing == nil {
		t.Fatalf("expected a kubectl call inside bash -c; got %+v", calls)
	}
	if !trailing.UncertainCwd {
		t.Errorf("expected UncertainCwd=true on trailing kubectl inside bash -c; got %+v", *trailing)
	}
}

func TestExtract_TerraformCheckThenSemicolonEcho_AllowsEcho(t *testing.T) {
	// Real-world case: `cd /repo && terraform fmt 2>&1; echo $?` —
	// walker emits both calls, terraform without UncertainCwd (cd's
	// RHS in AndStmt is reliable), echo with UncertainCwd=true.
	calls, refuse, _ := Extract(`cd /repo && terraform fmt 2>&1; echo done`)
	if refuse != "" {
		t.Fatalf("expected no walker refuse; got: %s", refuse)
	}
	// Two calls expected: terraform (cwd=/repo, UncertainCwd=false)
	// and echo (UncertainCwd=true, cwd=whatever the walker tracked).
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %+v", calls)
	}
	// Find each by name.
	var tf, ec *EffectiveCall
	for i := range calls {
		switch calls[i].Name {
		case "terraform":
			tf = &calls[i]
		case "echo":
			ec = &calls[i]
		}
	}
	if tf == nil || ec == nil {
		t.Fatalf("expected terraform + echo calls; got %+v", calls)
	}
	if tf.UncertainCwd {
		t.Errorf("terraform should NOT be UncertainCwd (it's the AndStmt RHS, cd-dependent)")
	}
	if !ec.UncertainCwd {
		t.Errorf("echo should be UncertainCwd (after `;` boundary)")
	}
}

func TestExtract_AndStmtCDStillSafeForItsOwnRHS(t *testing.T) {
	calls, refuse, _ := Extract("cd /repo && kubectl get pods")
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Cwd != "/repo" {
		t.Errorf("expected kubectl with Cwd=/repo, got %+v", calls)
	}
}

func TestExtract_AndStmtCDInAllowsTrailingNonCallCommand(t *testing.T) {
	_, refuse, _ := Extract("cd /tmp && true")
	if refuse != "" {
		t.Errorf("standalone cd-and-true should not refuse: %s", refuse)
	}
}

func TestExtract_RefuseUnquotedGlob(t *testing.T) {
	for _, cmd := range []string{
		`kubectl apply -f *.yaml`,
		`kubectl apply -f manifests/*.yaml`,
		`kubectl apply -f manifest?.yaml`,
		`kubectl apply -f manifest[0-9].yaml`,
		`kubectl apply -f {a,b}.yaml`,
		`rm -rf /tmp/*`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (unquoted glob)", cmd)
		}
	}
}

func TestExtract_QuotedGlobIsLiteral(t *testing.T) {
	calls, refuse, _ := Extract(`kubectl apply -f '*.yaml'`)
	if refuse != "" {
		t.Fatalf("quoted glob should NOT refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Args[len(calls[0].Args)-1] != "*.yaml" {
		t.Errorf("expected literal *.yaml in args, got %+v", calls)
	}
}

func TestExtract_RefuseUnquotedLeadingTilde(t *testing.T) {
	for _, cmd := range []string{
		`kubectl apply -f ~/manifests.yaml`,
		`cat ~someuser/file`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (unquoted leading tilde)", cmd)
		}
	}
}

func TestExtract_TildeInMiddleIsLiteral(t *testing.T) {
	calls, refuse, _ := Extract(`echo file~backup`)
	if refuse != "" {
		t.Errorf("mid-word tilde should not refuse: %s", refuse)
	}
	if len(calls) != 1 {
		t.Errorf("calls = %+v", calls)
	}
}

func TestExtract_RefuseInputRedirection(t *testing.T) {
	for _, cmd := range []string{
		`kubectl apply -f - < prod.yaml`,
		`cat < /etc/passwd`,
		`kubectl apply -f - <<< "$(cat prod.yaml)"`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (input redirection)", cmd)
		}
	}
}

func TestExtract_OutputRedirectionAllowed(t *testing.T) {
	calls, refuse, _ := Extract(`kubectl get pods > /tmp/out`)
	if refuse != "" {
		t.Fatalf("output redirection should not refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("expected kubectl call, got %+v", calls)
	}
}

func TestExtract_RefuseBlockLevelInputRedirection(t *testing.T) {
	for _, cmd := range []string{
		`{ kubectl apply -f -; } < prod.yaml`,
		`{ cd ./repo && kubectl apply -f -; } < prod.yaml`,
		`{ cat /etc/hosts; } < /dev/null`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (block-level < attached to outer stmt)", cmd)
		}
	}
}

func TestExtract_RefuseEnvPrefixTildeValue(t *testing.T) {
	for _, cmd := range []string{
		`KUBECONFIG=~/prod kubectl get pods`,
		`env KUBECONFIG=~/prod kubectl get pods`,
		`PATH=/usr/bin:~/bin kubectl get pods`,
		`env PATH=/usr/bin:~/bin kubectl get pods`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (unquoted ~ in env-prefix value)", cmd)
		}
	}
}

func TestExtract_QuotedTildeInEnvValueOK(t *testing.T) {
	calls, refuse, _ := Extract(`KUBECONFIG="~/prod" kubectl get pods`)
	if refuse != "" {
		t.Errorf("quoted tilde in env value should not refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Env["KUBECONFIG"] != "~/prod" {
		t.Errorf("expected literal ~/prod in env, got %+v", calls)
	}
}

func TestExtract_RefuseCDBareCDDashCDTilde(t *testing.T) {
	for _, cmd := range []string{
		`cd && kubectl get pods`,
		`cd -`,
		`cd - && kubectl get pods`,
		`cd ~`,
		`cd ~/Code && kubectl get pods`,
		`cd ~user && kubectl get pods`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (statically unknown cd target)", cmd)
		}
	}
}

func TestExtract_RefuseCDBareRelative(t *testing.T) {
	_, refuse, _ := Extract("cd subdir && kubectl get pods")
	if refuse == "" {
		t.Errorf("expected refuse on bare-relative cd target")
	}
}

func TestExtract_RelativeCDExplicitDotSlashIsFine(t *testing.T) {
	calls, refuse, _ := Extract("cd ./subdir && kubectl get pods")
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %+v", calls)
	}
	if calls[0].Cwd != "./subdir" {
		t.Errorf("Cwd = %q; the hook must Join(in.CWD, this) to resolve", calls[0].Cwd)
	}
}

func TestExtract_RefuseCDWithCDPATHEnvPrefix(t *testing.T) {
	for _, cmd := range []string{
		`CDPATH=/evil cd ./repo && kubectl apply -f x.yaml`,
		`CDPATH=/x:/y cd /abs/path && kubectl apply`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (CDPATH env-prefix)", cmd)
		}
	}
}

func TestExtract_RefuseFunctionDecl(t *testing.T) {
	_, refuse, _ := Extract(`f() { kubectl apply; }; f`)
	if refuse == "" {
		t.Errorf("expected refuse for function definition")
	}
}

func TestExtract_RefuseLoop(t *testing.T) {
	_, refuse, _ := Extract(`for i in 1 2; do kubectl apply -f $i.yaml; done`)
	if refuse == "" {
		t.Errorf("expected refuse for for loop")
	}
}

func TestExtract_AbsolutePath(t *testing.T) {
	calls, _, _ := Extract("/usr/local/bin/kubectl get pods")
	if calls[0].Name != "/usr/local/bin/kubectl" {
		t.Errorf("name = %q", calls[0].Name)
	}
}

func TestExtract_ChainedSemicolon(t *testing.T) {
	calls, _, _ := Extract("echo a; kubectl get pods")
	if len(calls) != 2 {
		t.Errorf("expected 2 calls (semicolon chain), got %+v", calls)
	}
}

func TestExtract_RefuseShellEvalBuiltins(t *testing.T) {
	for _, cmd := range []string{
		`eval "kubectl apply -f x.yaml"`,
		`source /tmp/setup.sh`,
		`. /tmp/setup.sh`,
		`exec kubectl apply -f x.yaml`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q", cmd)
		}
	}
}

func TestExtract_RefuseHeredoc(t *testing.T) {
	cmd := "bash <<'EOF'\nkubectl apply -f x.yaml\nEOF\n"
	_, refuse, _ := Extract(cmd)
	if refuse == "" {
		t.Errorf("expected refuse on heredoc")
	}
}

func TestExtract_AbsolutePathBashDashC(t *testing.T) {
	calls, refuse, _ := Extract(`/bin/bash -c "kubectl apply -f x.yaml"`)
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("expected unwrapped kubectl call from /bin/bash -c, got %+v", calls)
	}
}

func TestExtract_AbsolutePathEnv(t *testing.T) {
	calls, refuse, _ := Extract(`/usr/bin/env kubectl get pods`)
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("expected unwrapped kubectl from /usr/bin/env, got %+v", calls)
	}
}

func TestExtract_RefuseShWithoutDashC(t *testing.T) {
	for _, cmd := range []string{
		`sh /tmp/script.sh`,
		`bash /tmp/script.sh`,
		`/bin/sh /tmp/script.sh`,
		`bash -i`,
		`sh -- foo bar`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q", cmd)
		}
	}
}

func TestExtract_RefuseShDashCWithExtraArgs(t *testing.T) {
	_, refuse, _ := Extract(`sh -c "kubectl get pods" myname extra`)
	if refuse == "" {
		t.Errorf("expected refuse on -c with trailing positional args")
	}
}

func TestExtract_RefuseEnvDashI(t *testing.T) {
	for _, cmd := range []string{
		`env -i kubectl apply -f x.yaml`,
		`env -u PATH kubectl apply`,
		`env -S "FOO=bar" kubectl apply`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q", cmd)
		}
	}
}

func TestExtract_RefuseTransparentWrapperWithFlags(t *testing.T) {
	for _, cmd := range []string{
		`nice -n 10 kubectl apply -f x.yaml`,
		`time -p kubectl apply -f x.yaml`,
		`ionice -c 3 kubectl apply -f x.yaml`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q", cmd)
		}
	}
	calls, _, _ := Extract(`nice kubectl get pods`)
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("bare nice should unwrap to kubectl: %+v", calls)
	}
}

func TestExtract_XargsPeelsBooleanFlags(t *testing.T) {
	cases := []struct {
		cmd      string
		wantName string
		wantArg0 string // first arg, which should be the literal first arg to inner CMD
	}{
		{`echo a b | xargs du -sh`, "du", "-sh"},
		{`echo a b | xargs -0 du -sh`, "du", "-sh"},
		{`echo a | xargs -r --null cat`, "cat", "<dynamic>"},
		{`echo a | xargs -t -x grep foo`, "grep", "foo"},
	}
	for _, tc := range cases {
		calls, refuse, _ := Extract(tc.cmd)
		if refuse != "" {
			t.Errorf("expected no refuse for %q; got: %s", tc.cmd, refuse)
			continue
		}
		// Find the inner-command call (skip echo).
		var inner *EffectiveCall
		for i := range calls {
			if calls[i].Name == tc.wantName {
				inner = &calls[i]
				break
			}
		}
		if inner == nil {
			t.Errorf("expected inner %s call for %q; got %+v", tc.wantName, tc.cmd, calls)
			continue
		}
		if len(inner.Args) == 0 || inner.Args[0] != tc.wantArg0 {
			t.Errorf("for %q: inner args = %v, want first arg = %q", tc.cmd, inner.Args, tc.wantArg0)
		}
	}
}

func TestExtract_XargsValueFlagRefuses(t *testing.T) {
	cases := []string{
		`xargs -I {} echo {}`,
		`xargs -n 1 echo`,
		`xargs -P 4 sh -c 'echo $0'`,
		`xargs -L 1 echo`,
		`xargs -s 100 echo`,
		`xargs -d ',' echo`,
		`xargs --replace=R echo`,
		`xargs --max-args=5 echo`,
	}
	for _, cmd := range cases {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for value-taking xargs flag in %q", cmd)
		}
	}
}

func TestExtract_XargsNoCommandRefuses(t *testing.T) {
	_, refuse, _ := Extract(`xargs`)
	if refuse == "" {
		t.Errorf("expected refuse for bare xargs")
	}
	_, refuse, _ = Extract(`xargs -0`)
	if refuse == "" {
		t.Errorf("expected refuse for xargs with only flags, no command")
	}
}

func TestExtract_DiskUsageFindXargsDuPipeline(t *testing.T) {
	cmd := `find /tmp -maxdepth 1 -print0 | xargs -0 du -sh 2>/dev/null | sort -rh | head -40`
	calls, refuse, _ := Extract(cmd)
	if refuse != "" {
		t.Fatalf("expected no refuse for find/xargs/du pipeline; got: %s", refuse)
	}
	// Should have find, du (from peeled xargs), sort, head — none are infra.
	names := make([]string, 0, len(calls))
	for _, c := range calls {
		names = append(names, c.Name)
	}
	wantContains := []string{"find", "du", "sort", "head"}
	for _, w := range wantContains {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in extracted calls; got %v", w, names)
		}
	}
}

func TestExtract_NestedEnvBashDashC(t *testing.T) {
	calls, refuse, _ := Extract(`env FOO=bar bash -c "kubectl apply -f x.yaml"`)
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Fatalf("expected kubectl, got %+v", calls)
	}
	if calls[0].Env["FOO"] != "bar" {
		t.Errorf("env-prefix did not propagate: %v", calls[0].Env)
	}
}

func TestExtract_NestedNiceBashDashC(t *testing.T) {
	calls, refuse, _ := Extract(`nice bash -c "kubectl apply"`)
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("expected kubectl, got %+v", calls)
	}
}

func TestExtract_DeeplyNestedWrappers(t *testing.T) {
	calls, refuse, _ := Extract(`env A=1 nice bash -c "kubectl get pods"`)
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Errorf("expected kubectl, got %+v", calls)
	}
	if calls[0].Env["A"] != "1" {
		t.Errorf("env-prefix did not propagate through layers: %v", calls[0].Env)
	}
}

func TestExtract_RefuseBashInteractiveOrLogin(t *testing.T) {
	for _, cmd := range []string{
		`bash -i -c "kubectl apply"`,
		`bash -l -c "kubectl apply"`,
		`bash -lc "kubectl apply"`,
		`bash --login -c "kubectl apply"`,
		`bash --interactive -c "kubectl apply"`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (interactive/login sources startup files)", cmd)
		}
	}
}

func TestExtract_RefuseStartupSourcingEnvVars(t *testing.T) {
	for _, cmd := range []string{
		`BASH_ENV=/tmp/setup.sh bash -c "kubectl apply"`,
		`ENV=/tmp/setup.sh sh -c "kubectl apply"`,
		`PROMPT_COMMAND="echo evil" bash -c "kubectl apply"`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q", cmd)
		}
	}
}

func TestExtract_CDInsideBashDashCComposes(t *testing.T) {
	calls, refuse, _ := Extract(`cd /repo && bash -c 'cd ./sub && kubectl apply'`)
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "kubectl" {
		t.Fatalf("expected one kubectl call, got %+v", calls)
	}
	if calls[0].Cwd != "/repo/sub" {
		t.Errorf("composed Cwd = %q, want /repo/sub", calls[0].Cwd)
	}
}

func TestExtract_AbsoluteCDInsideBashDashCReplaces(t *testing.T) {
	calls, _, _ := Extract(`cd /repo && bash -c 'cd /elsewhere && kubectl apply'`)
	if len(calls) != 1 || calls[0].Cwd != "/elsewhere" {
		t.Errorf("expected /elsewhere, got %+v", calls)
	}
}

func TestExtract_RefuseShellStateBuiltins(t *testing.T) {
	for _, cmd := range []string{
		`export KUBECONFIG=/tmp/x; kubectl apply`,
		`export KUBECONFIG=/tmp/x && kubectl apply`,
		`unset HOME; kubectl apply`,
		`set -e; kubectl apply`,
		`alias k=kubectl; k apply`,
		`pushd /tmp; kubectl apply`,
		`trap 'echo done' EXIT; kubectl apply`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q", cmd)
		}
	}
}

func TestExtract_BuiltinPeels(t *testing.T) {
	calls, refuse, _ := Extract(`builtin cat /etc/hosts`)
	if refuse != "" {
		t.Fatalf("unexpected refuse: %s", refuse)
	}
	if len(calls) != 1 || calls[0].Name != "cat" {
		t.Errorf("expected cat call after builtin peel, got %+v", calls)
	}
}

func TestExtract_BuiltinEscapeRefuses(t *testing.T) {
	for _, cmd := range []string{
		`builtin eval "kubectl apply"`,
		`builtin source ./x`,
		`builtin . ./x`,
		`builtin export KUBECONFIG=/tmp/x; kubectl apply`,
		`builtin unset PATH`,
		`builtin alias k=kubectl`,
	} {
		_, refuse, _ := Extract(cmd)
		if refuse == "" {
			t.Errorf("expected refuse for %q (builtin escape via inner eval/state-mutation)", cmd)
		}
	}
}
