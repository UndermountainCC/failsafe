// Copyright 2026 Undermountain Coding Company
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/UndermountainCC/failsafe/internal/config"
)

// TestTelemetryDefaultOff verifies that the default TelemetryConfig (Enabled=false)
// returns a no-op exporter that accepts Export calls without panicking or blocking.
func TestTelemetryDefaultOff(t *testing.T) {
	cfg := config.TelemetryConfig{
		Enabled:      false,
		OTLPEndpoint: "",
	}
	exp := New(cfg)
	if exp == nil {
		t.Fatal("New returned nil exporter for disabled config")
	}
	// Must not panic, block, or produce any side effects.
	exp.Export(Event{
		Decision: "allow",
		Tool:     "kubectl",
		Verb:     "get",
		Mode:     "enabled",
	})
}

// TestTelemetryEnabledIsStub verifies that enabling telemetry in v1 still returns
// a stub (no-op) exporter — no real OTLP export occurs.
func TestTelemetryEnabledIsStub(t *testing.T) {
	cfg := config.TelemetryConfig{
		Enabled:      true,
		OTLPEndpoint: "http://localhost:4317",
	}
	exp := New(cfg)
	if exp == nil {
		t.Fatal("New returned nil exporter for enabled config")
	}
	// Must not panic or attempt a network connection.
	exp.Export(Event{
		Decision: "block",
		Tool:     "terraform",
		Verb:     "apply",
		Mode:     "enabled",
	})
}

// TestExporterTypeDefaultOff confirms that the disabled path returns the no-op
// type, not the stub — this documents the intended type distinction.
func TestExporterTypeDefaultOff(t *testing.T) {
	exp := New(config.TelemetryConfig{Enabled: false})
	if _, ok := exp.(noopExporter); !ok {
		t.Errorf("disabled telemetry: expected noopExporter, got %T", exp)
	}
}

// TestExporterTypeEnabledIsStub confirms that the enabled path returns the stub
// type in v1.
func TestExporterTypeEnabledIsStub(t *testing.T) {
	exp := New(config.TelemetryConfig{Enabled: true, OTLPEndpoint: "http://localhost:4317"})
	if _, ok := exp.(*stubExporter); !ok {
		t.Errorf("enabled telemetry: expected *stubExporter, got %T", exp)
	}
}
