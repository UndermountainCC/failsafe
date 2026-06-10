// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

// Package telemetry provides a minimal, opt-in telemetry interface for failsafe.
//
// # v1 status: stub exporter only
//
// Telemetry is OFF by default (TelemetryConfig.Enabled = false).  When disabled,
// New returns a no-op exporter that discards all events without any I/O.
//
// When enabled (opt-in via config), the returned exporter is still a stub in v1 —
// it accepts events and discards them, but includes the intended OTLP payload shape
// as documentation so it can be code-reviewed before a real backend ships.
//
// # Intended payload (v2 OTLP exporter — not implemented in v1)
//
// When a real exporter ships it will emit OTLP spans/events to OTLPEndpoint with
// the following attributes:
//
//	span name:           "failsafe.hook"
//	failsafe.decision:   "block" | "allow" | "allow_override"
//	failsafe.tool:       registry tool name (e.g. "kubectl", "terraform")
//	failsafe.verb:       parsed verb (e.g. "apply", "destroy")  — omitted if empty
//	failsafe.subverb:    parsed subverb — omitted if empty
//	failsafe.mode:       "enabled" | "disabled"
//	failsafe.version:    failsafe binary version string
//
// No user-identifying data (hostname, username, file paths, command arguments,
// session IDs) is included.  The payload is intentionally narrow: aggregate
// decision counts by tool/verb/mode, nothing else.
//
// # No OTLP dependency in v1
//
// This package does not import any OpenTelemetry SDK.  When the real exporter
// is added it will live in a separate build tag or sub-package so users who
// never enable telemetry incur zero binary size cost.
package telemetry

import "github.com/UndermountainCC/failsafe/internal/config"

// ---------------------------------------------------------------------------
// Event type
// ---------------------------------------------------------------------------

// Event represents a single hook decision to be exported.
type Event struct {
	// Decision is the outcome: "block", "allow", or "allow_override".
	Decision string
	// Tool is the registry tool name (e.g. "kubectl").  May be empty for
	// non-tool decisions (e.g. parse failures).
	Tool string
	// Verb is the parsed verb.  May be empty.
	Verb string
	// Subverb is the parsed subverb.  May be empty.
	Subverb string
	// Mode is the failsafe mode at decision time: "enabled" or "disabled".
	Mode string
}

// ---------------------------------------------------------------------------
// Exporter interface
// ---------------------------------------------------------------------------

// Exporter is the telemetry output interface.  Implementations must be safe
// for concurrent use.
type Exporter interface {
	// Export records an event.  The call is best-effort: implementations must
	// not block the hook path; they should drop events rather than stall.
	Export(e Event)
}

// ---------------------------------------------------------------------------
// No-op exporter
// ---------------------------------------------------------------------------

// noopExporter silently discards all events.
type noopExporter struct{}

func (noopExporter) Export(Event) {}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// New returns an Exporter appropriate for cfg.
//
//   - cfg.Enabled == false (the default): returns a no-op exporter.  No I/O,
//     no goroutines, no dependencies.
//
//   - cfg.Enabled == true: returns a stub exporter (v1).  Events are accepted
//     and discarded.  A real OTLP exporter will replace this stub in a future
//     release; see the package-level doc comment for the intended payload shape.
//
// TODO: real OTLP exporter — replace stubExporter with an OTLP batch exporter
// that sends spans to cfg.OTLPEndpoint using the OpenTelemetry Go SDK.
func New(cfg config.TelemetryConfig) Exporter {
	if !cfg.Enabled {
		return noopExporter{}
	}
	// v1 stub: telemetry is configured but the real exporter hasn't shipped yet.
	// Events are silently dropped.  cfg.OTLPEndpoint is recorded here for when
	// the real exporter is wired up.
	return &stubExporter{endpoint: cfg.OTLPEndpoint}
}

// stubExporter is the v1 "enabled but no-op" exporter.  It accepts events and
// discards them, but documents where the real exporter will deliver them.
type stubExporter struct {
	endpoint string // target OTLP endpoint — unused until real exporter ships
}

func (s *stubExporter) Export(Event) {
	// TODO: real OTLP exporter — batch-send events to s.endpoint.
	// The intended payload shape is documented in the package-level doc comment.
}
