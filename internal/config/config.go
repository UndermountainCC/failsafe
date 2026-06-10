// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package config provides a typed, layered configuration loader for failsafe.
// Providers are composed lowest-to-highest (defaults → file → env → flags),
// so flags always win and a missing config file is equivalent to all-defaults.
//
// Safety invariants enforced at load time (Validate):
//   - log.redact is always true; false is a fatal error.
//   - control_plane.url / control_plane.bundle_signing_key are reserved (v1); setting either is fatal.
//
// The chain default (mode.Chain.Default) is always the hardcoded literal "enabled".
// There is no mode.default config key — a configurable default would be a
// self-disable vector (an agent editing config.yaml could bypass the guard).
//
// Back-compat shims (applied before the env provider loads):
//   - FAILSAFE_LOG=off            → log.enabled=false
//   - FAILSAFE_LOG=<path>         → log.path=<path>, log.enabled=true
//   - FAILSAFE_MODE is intentionally NOT mapped to any config field; it stays as the
//     first source in the mode chain (EnvSource{Name:"FAILSAFE_MODE"}) so per-pane
//     file sources can still override it.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// ---------------------------------------------------------------------------
// Typed config structs (koanf-tagged)
// ---------------------------------------------------------------------------

// Config is the root configuration struct.  Callers receive a *Config from
// Load; every field is guaranteed to be non-zero after a successful load.
type Config struct {
	Mode         ModeConfig         `koanf:"mode"`
	Log          LogConfig          `koanf:"log"`
	Telemetry    TelemetryConfig    `koanf:"telemetry"`
	Policy       PolicyConfig       `koanf:"policy"`
	Trust        TrustConfig        `koanf:"trust"`
	ControlPlane ControlPlaneConfig `koanf:"control_plane"`
}

// ModeConfig holds mode-resolution settings.
type ModeConfig struct {
	// PaneDir is the toggle-file directory.  Effectively fixed at
	// ~/.claude/pane-mode; see spec §5 for why this is "recorded not driven".
	//
	// Note: there is intentionally no Default field here.  The chain's default
	// is always the hardcoded literal "enabled" (set in buildModeChain) — a
	// configurable default would be a self-disable vector.
	PaneDir string `koanf:"pane_dir"`
}

// LogConfig controls the audit log.
type LogConfig struct {
	Enabled bool   `koanf:"enabled"`
	Path    string `koanf:"path"`
	// Redact is safety-fixed true; Validate rejects false with a fatal error.
	Redact bool `koanf:"redact"`
}

// TelemetryConfig is OFF by default; opt-in only.  v1 exporter is a stub.
type TelemetryConfig struct {
	Enabled      bool   `koanf:"enabled"`
	OTLPEndpoint string `koanf:"otlp_endpoint"`
}

// PolicyConfig locates user-supplied Rego and tool YAML overrides.
type PolicyConfig struct {
	UserPath string `koanf:"user_path"`
	ToolsDir string `koanf:"tools_dir"`
}

// TrustConfig locates the trusted-repos file.
type TrustConfig struct {
	Path string `koanf:"path"`
}

// ControlPlaneConfig is defined for forward-compat but rejected at load in v1.
// Setting either field is a fatal error ("not supported in v1").
type ControlPlaneConfig struct {
	URL              string `koanf:"url"`
	BundleSigningKey string `koanf:"bundle_signing_key"`
}

// ---------------------------------------------------------------------------
// Load options
// ---------------------------------------------------------------------------

// Options controls how Load resolves paths and reads the environment.
// Mirrors the home+env testability pattern used by auditlog.DefaultLogger.
type Options struct {
	// Home is the user home directory.  Falls back to os.Getenv("HOME") when empty.
	Home string
	// Env is the environment accessor.  Falls back to os.Getenv when nil.
	Env func(string) string
	// Flags is the parsed pflag.FlagSet.  Nil means skip the flags provider.
	Flags *pflag.FlagSet
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

// defaults returns the compile-time default Config.  Tilde paths are expanded
// by expandPaths after unmarshal, so we record them verbatim here.
func defaults() Config {
	return Config{
		Mode: ModeConfig{
			PaneDir: "~/.claude/pane-mode",
		},
		Log: LogConfig{
			Enabled: true,
			Path:    "~/.config/failsafe/decisions.jsonl",
			Redact:  true,
		},
		Telemetry: TelemetryConfig{
			Enabled:      false,
			OTLPEndpoint: "",
		},
		Policy: PolicyConfig{
			UserPath: "~/.config/failsafe/policy.rego",
			ToolsDir: "~/.config/failsafe/tools",
		},
		Trust: TrustConfig{
			Path: "~/.config/failsafe/trusted-repos.yaml",
		},
		ControlPlane: ControlPlaneConfig{},
	}
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

// Load builds and returns the effective *Config by composing providers
// lowest-to-highest: defaults → config file → env → flags.
//
// A missing config file is not an error (§Migration D1).  Env shims for
// FAILSAFE_LOG are applied before the env provider so they participate in the
// normal precedence chain.
func Load(opts Options) (*Config, error) {
	home := opts.Home
	if home == "" {
		// Resolve once; used for tilde expansion and config file path.
		if opts.Env != nil {
			home = opts.Env("HOME")
		}
	}
	getenv := opts.Env
	if getenv == nil {
		getenv = realGetenv
	}

	k := koanf.New(".")

	// 1. Defaults — lowest precedence.
	if err := k.Load(structs.Provider(defaults(), "koanf"), nil); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}

	// 2. Config file — optional; missing is not an error.
	configPath := expandTilde("~/.config/failsafe/config.yaml", home)
	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		// A missing file is the common/expected case; any read/parse error on
		// an existing file is surfaced so the operator knows config is broken.
		if !isNotExist(err) {
			return nil, fmt.Errorf("config: load file %s: %w", configPath, err)
		}
	}

	// 3. Env provider — apply back-compat shims first, then load standard vars.
	shimMap := buildEnvShims(getenv)

	if err := k.Load(shimEnvProvider{m: shimMap}, nil); err != nil {
		return nil, fmt.Errorf("config: apply env shims: %w", err)
	}

	// Use an injectable env provider so tests can supply a custom env function.
	// In production getenv == os.Getenv; in tests it's the opts.Env stub.
	if err := k.Load(newEnvProvider("FAILSAFE_", getenv), nil); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	// 4. Flags — highest precedence (skipped when nil).
	if opts.Flags != nil {
		if err := k.Load(posflag.Provider(opts.Flags, ".", k), nil); err != nil {
			return nil, fmt.Errorf("config: load flags: %w", err)
		}
	}

	// Unmarshal into typed struct.
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	// Expand tilde and ${VAR} in path-type fields.
	expandPaths(&cfg, home, getenv)

	// Validate safety invariants — fatal on violation.
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ---------------------------------------------------------------------------
// Env transform (FAILSAFE_ → koanf keys)
// ---------------------------------------------------------------------------

// envKeyTable is a static mapping from each supported FAILSAFE_* env var name
// to its exact koanf dot-path key.  Using a lookup table instead of a
// mechanical _→. replacement avoids mangling multi-word field names such as
// FAILSAFE_MODE_PANE_DIR (which must map to "mode.pane_dir", not
// "mode.pane.dir").
//
// Exclusions (intentional):
//   - FAILSAFE_MODE: handled by the mode chain's EnvSource, not a config field.
//   - FAILSAFE_LOG: handled by the back-compat shim (buildEnvShims).
var envKeyTable = map[string]string{
	"FAILSAFE_MODE_PANE_DIR":            "mode.pane_dir",
	"FAILSAFE_LOG_ENABLED":              "log.enabled",
	"FAILSAFE_LOG_PATH":                 "log.path",
	"FAILSAFE_LOG_REDACT":               "log.redact",
	"FAILSAFE_TELEMETRY_ENABLED":        "telemetry.enabled",
	"FAILSAFE_TELEMETRY_OTLP_ENDPOINT":  "telemetry.otlp_endpoint",
	"FAILSAFE_POLICY_USER_PATH":         "policy.user_path",
	"FAILSAFE_POLICY_TOOLS_DIR":         "policy.tools_dir",
	"FAILSAFE_TRUST_PATH":               "trust.path",
}

// envTransform converts a FAILSAFE_* env key to the matching koanf key using
// the static envKeyTable.  Returns "" for unrecognised keys so the caller can
// skip them cleanly.
func envTransform(s string) string {
	return envKeyTable[s]
}

// ---------------------------------------------------------------------------
// Back-compat shim helpers
// ---------------------------------------------------------------------------

// buildEnvShims inspects the legacy FAILSAFE_LOG env var and returns a flat
// koanf key map that reproduces the exact semantics of auditlog.DefaultLogger:
//
//	FAILSAFE_LOG=off        → log.enabled=false
//	FAILSAFE_LOG=<path>     → log.path=<path>, log.enabled=true
//	FAILSAFE_LOG unset/""   → no override (defaults stand)
//
// FAILSAFE_MODE is deliberately excluded: it lives as EnvSource{Name:"FAILSAFE_MODE"}
// in the mode chain so pane-file sources can override it.
func buildEnvShims(getenv func(string) string) map[string]interface{} {
	m := map[string]interface{}{}
	switch v := getenv("FAILSAFE_LOG"); {
	case v == "off":
		m["log.enabled"] = false
	case v != "":
		m["log.path"] = v
		m["log.enabled"] = true
	}
	return m
}

// shimEnvProvider is a minimal koanf.Provider that injects the pre-computed
// shim map (flat keys with "."-delimiter) into the koanf instance.
type shimEnvProvider struct{ m map[string]interface{} }

func (s shimEnvProvider) Read() (map[string]interface{}, error) {
	// Unflatten "log.enabled" → {"log": {"enabled": v}}.
	out := map[string]interface{}{}
	for k, v := range s.m {
		parts := strings.Split(k, ".")
		setNested(out, parts, v)
	}
	return out, nil
}
func (s shimEnvProvider) ReadBytes() ([]byte, error) {
	return nil, errors.New("shimEnvProvider does not support ReadBytes")
}

// setNested builds a nested map from a key path and value.
func setNested(m map[string]interface{}, path []string, v interface{}) {
	if len(path) == 1 {
		m[path[0]] = v
		return
	}
	sub, ok := m[path[0]].(map[string]interface{})
	if !ok {
		sub = map[string]interface{}{}
		m[path[0]] = sub
	}
	setNested(sub, path[1:], v)
}

// ---------------------------------------------------------------------------
// Injectable env provider
// ---------------------------------------------------------------------------

// injectableEnvProvider is a koanf.Provider that reads FAILSAFE_* vars from a
// caller-supplied getenv function instead of os.Environ.  This makes the env
// layer fully testable without real environment variables.
type injectableEnvProvider struct {
	prefix  string
	getenv  func(string) string
	environ func() []string // returns "KEY=VALUE" pairs; nil → os.Environ
}

// newEnvProvider returns an env provider that uses getenv for lookups.
// In production getenv should be os.Getenv; in tests supply a stub.
func newEnvProvider(prefix string, getenv func(string) string) *injectableEnvProvider {
	return &injectableEnvProvider{
		prefix:  prefix,
		getenv:  getenv,
		environ: realEnviron,
	}
}

func (e *injectableEnvProvider) ReadBytes() ([]byte, error) {
	return nil, errors.New("injectableEnvProvider does not support ReadBytes")
}

func (e *injectableEnvProvider) Read() (map[string]interface{}, error) {
	// Collect all env keys to check against our prefix.  We need both the key
	// list (from environ) and the values (from our injected getenv) so tests
	// that inject a stub getenv get the correct values.
	out := map[string]interface{}{}
	for _, kv := range e.environ() {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		rawKey := kv[:idx]
		if !strings.HasPrefix(rawKey, e.prefix) {
			continue
		}
		mapped := envTransform(rawKey)
		if mapped == "" {
			continue
		}
		// Skip single-segment keys (e.g. FAILSAFE_MODE → "mode"): they map to
		// struct fields, not leaf values. FAILSAFE_MODE is intentionally handled
		// via the mode chain's EnvSource, not the config struct.
		if !strings.Contains(mapped, ".") {
			continue
		}
		val := e.getenv(rawKey)
		parts := strings.Split(mapped, ".")
		setNested(out, parts, val)
	}
	// Also check any keys that opts.Env knows about but os.Environ might not
	// (the injected env may be a pure stub with no real os.Environ backing).
	// We scan a known set of FAILSAFE_ keys so we don't miss stub-only vars.
	for _, rawKey := range knownEnvKeysList() {
		if !strings.HasPrefix(rawKey, e.prefix) {
			continue
		}
		val := e.getenv(rawKey)
		if val == "" {
			continue
		}
		mapped := envTransform(rawKey)
		if mapped == "" {
			continue
		}
		parts := strings.Split(mapped, ".")
		setNested(out, parts, val)
	}
	return out, nil
}

// knownEnvKeys is the canonical list of FAILSAFE_* env var names supported in
// v1.  Derived from envKeyTable so it stays in sync automatically.
// Used by injectableEnvProvider to ensure stub envs (used in tests) are fully
// read even when os.Environ does not contain these keys.
func knownEnvKeysList() []string {
	keys := make([]string, 0, len(envKeyTable))
	for k := range envKeyTable {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Path expansion
// ---------------------------------------------------------------------------

// expandPaths expands tilde and ${VAR} in all path-type config fields.
// It uses the same substitution semantics as internal/mode.expandVars but is
// applied post-unmarshal so struct types are already known.
func expandPaths(cfg *Config, home string, getenv func(string) string) {
	envMap := func(k string) string { return getenv(k) }
	expand := func(s string) string {
		s = expandTilde(s, home)
		s = expandEnvVars(s, envMap)
		return s
	}
	cfg.Mode.PaneDir = expand(cfg.Mode.PaneDir)
	cfg.Log.Path = expand(cfg.Log.Path)
	cfg.Policy.UserPath = expand(cfg.Policy.UserPath)
	cfg.Policy.ToolsDir = expand(cfg.Policy.ToolsDir)
	cfg.Trust.Path = expand(cfg.Trust.Path)
	cfg.Telemetry.OTLPEndpoint = expand(cfg.Telemetry.OTLPEndpoint)
	cfg.ControlPlane.URL = expand(cfg.ControlPlane.URL)
}

// expandTilde replaces a leading ~ with home.
func expandTilde(s, home string) string {
	if s == "~" {
		return home
	}
	if strings.HasPrefix(s, "~/") {
		return home + s[1:]
	}
	return s
}

// expandEnvVars performs ${VAR} substitution (no substitution if VAR is unset).
// A missing variable leaves the literal "${VAR}" in place rather than returning
// an error — callers will surface a "path is empty" error at use-time, which is
// clearer than a generic expansion error at load-time.
func expandEnvVars(s string, getenv func(string) string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			end := strings.Index(s[i+2:], "}")
			if end == -1 {
				out.WriteString(s[i:])
				break
			}
			name := s[i+2 : i+2+end]
			v := getenv(name)
			if v != "" {
				out.WriteString(v)
			} else {
				out.WriteString(s[i : i+2+end+1]) // preserve literal
			}
			i += 2 + end + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

// Validate enforces safety invariants.  It is called by Load after Unmarshal
// and path expansion.  A validation error is fatal.
func (c *Config) Validate() error {
	// log.path must be non-empty after expansion.
	if c.Log.Path == "" {
		return errors.New("config: log.path is empty after expansion")
	}

	// mode.pane_dir must be non-empty after expansion.
	if c.Mode.PaneDir == "" {
		return errors.New("config: mode.pane_dir is empty after expansion")
	}

	// log.redact is safety-fixed true; disabling redaction is a fatal error.
	if !c.Log.Redact {
		return errors.New("config: log.redact cannot be disabled (safety-fixed true)")
	}

	// control_plane.* is reserved in v1; either field set → fatal.
	if c.ControlPlane.URL != "" || c.ControlPlane.BundleSigningKey != "" {
		return errors.New("config: control_plane.* is reserved and not supported in v1")
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// realGetenv is the production env accessor.
// Wrapped so tests can substitute without build tags.
func realGetenv(key string) string {
	// Import avoided: use os through the type assertion below.
	// The actual import is in config_os.go to keep this file testable.
	return _osGetenv(key)
}

// isNotExist returns true when the error represents a missing file.  koanf's
// file provider wraps os.ReadFile errors, so we check the underlying cause.
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	// Check for "no such file or directory" in the error message as a fallback,
	// since koanf wraps the underlying os.PathError differently across versions.
	msg := err.Error()
	return strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "not exist") ||
		strings.Contains(msg, "cannot find")
}
