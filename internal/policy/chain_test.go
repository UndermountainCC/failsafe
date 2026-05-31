// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChain_BundledOnly(t *testing.T) {
	mods, err := Discover(DiscoverOpts{
		BundledLoader: func() ([]Module, error) {
			return []Module{{Layer: LayerBundled, File: "x.rego", Body: `package guard.bundled.x`}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].Layer != LayerBundled {
		t.Errorf("expected 1 bundled module, got %v", mods)
	}
}

func TestChain_UserAndRepo_TrustedTagging(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(home, "Code", "myrepo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config", "failsafe"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(home, ".config", "failsafe", "policy.rego"),
		[]byte(`package guard.user`), 0o644)
	os.WriteFile(filepath.Join(repo, ".failsafe.rego"),
		[]byte(`package guard.repo`), 0o644)

	mods, _ := Discover(DiscoverOpts{
		BundledLoader: func() ([]Module, error) {
			return []Module{{Layer: LayerBundled, File: "x.rego", Body: `package guard.bundled.x`}}, nil
		},
		Home: home,
		CWD:  repo,
		IsTrusted: func(p string) bool {
			return p == repo // trust this exact repo
		},
	})

	var bundled, user, repoMod *Module
	for i := range mods {
		switch mods[i].Layer {
		case LayerBundled:
			bundled = &mods[i]
		case LayerUser:
			user = &mods[i]
		case LayerRepo:
			repoMod = &mods[i]
		}
	}
	if bundled == nil || user == nil || repoMod == nil {
		t.Fatalf("expected all three layers; got %+v", mods)
	}
	if !repoMod.Trusted {
		t.Errorf("repo should be tagged Trusted=true (IsTrusted returned true)")
	}
}

func TestChain_UntrustedRepoWithOverride_Warns(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(home, "Code", "myrepo")
	os.MkdirAll(repo, 0o755)
	os.WriteFile(filepath.Join(repo, ".failsafe.rego"), []byte(`package guard.repo

import future.keywords.if
import future.keywords.contains

allow_override contains {"reason": "demo"} if { true }
`), 0o644)

	var warnedRepo, warnedFile string
	mods, _ := Discover(DiscoverOpts{
		BundledLoader: func() ([]Module, error) { return nil, nil },
		Home:          home,
		CWD:           repo,
		IsTrusted:     func(p string) bool { return false }, // untrusted
		WarnUntrusted: func(repoPath, file string) {
			warnedRepo = repoPath
			warnedFile = file
		},
	})
	var repoMod *Module
	for i := range mods {
		if mods[i].Layer == LayerRepo {
			repoMod = &mods[i]
		}
	}
	if repoMod == nil {
		t.Fatal("expected repo module to still be present (block rules survive)")
	}
	if repoMod.Trusted {
		t.Errorf("repo should be Trusted=false")
	}
	if warnedRepo == "" || !strings.HasSuffix(warnedFile, ".failsafe.rego") {
		t.Errorf("expected WarnUntrusted called; got repo=%q file=%q", warnedRepo, warnedFile)
	}
}

func TestChain_UntrustedRepoNoOverride_NoWarning(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	repo := filepath.Join(home, "Code", "myrepo")
	os.MkdirAll(repo, 0o755)
	os.WriteFile(filepath.Join(repo, ".failsafe.rego"), []byte(`package guard.repo

import future.keywords.if
import future.keywords.contains

block contains {"reason": "x"} if { true }
`), 0o644)

	warnings := 0
	Discover(DiscoverOpts{
		BundledLoader: func() ([]Module, error) { return nil, nil },
		Home:          home,
		CWD:           repo,
		IsTrusted:     func(p string) bool { return false },
		WarnUntrusted: func(_, _ string) { warnings++ },
	})
	if warnings != 0 {
		t.Errorf("warnings = %d; should be 0 when repo has only block rules", warnings)
	}
}

// User policy with permission-denied (or any non-ENOENT read error) MUST
// fail closed — silently skipping an existing policy we can't read is
// fail-open. (chmod 0o000 may not enforce on every filesystem; if the
// runner's FS ignores mode bits the test will be skipped via stat probe.)
func TestDiscover_UserPolicyPermissionDeniedFailsClosed(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read anything; skip permission test")
	}
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	cfg := filepath.Join(home, ".config", "failsafe")
	os.MkdirAll(cfg, 0o755)
	p := filepath.Join(cfg, "policy.rego")
	os.WriteFile(p, []byte(`package guard.user`), 0o644)
	// Make file unreadable.
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(p, 0o644) // for tempdir cleanup

	// Probe: confirm chmod actually denies reads on this FS. Some CI
	// environments (Docker on certain mounts) ignore mode bits.
	if _, err := os.ReadFile(p); err == nil {
		t.Skip("filesystem ignores chmod 0o000; cannot test permission-denied path")
	}

	_, err := Discover(DiscoverOpts{
		BundledLoader: func() ([]Module, error) { return nil, nil },
		Home:          home,
	})
	if err == nil {
		t.Error("expected error on unreadable user policy; permission-denied must fail closed")
	}
}

func TestChain_RepoStopsAtHome(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	os.MkdirAll(home, 0o755)
	os.WriteFile(filepath.Join(home, ".failsafe.rego"), []byte(`package guard.repo`), 0o644)
	subdir := filepath.Join(home, "Code", "x")
	os.MkdirAll(subdir, 0o755)

	mods, _ := Discover(DiscoverOpts{
		BundledLoader: func() ([]Module, error) { return nil, nil },
		Home:          home,
		CWD:           subdir,
	})
	for _, m := range mods {
		if m.Layer == LayerRepo {
			t.Errorf("did not expect repo layer; got %s (file=%s)", m.Layer, m.File)
		}
	}
}
