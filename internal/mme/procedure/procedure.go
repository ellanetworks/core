// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure defines the MME's EPS key-changing procedure type set, binding
// it to the shared procedure engine (internal/procedure).
package procedure

import (
	engine "github.com/ellanetworks/core/internal/procedure"
	"go.uber.org/zap"
)

// Type and the registry are re-exported from the engine so MME code refers to a
// single procedure package.
type (
	Type     = engine.Type
	Registry = engine.Registry
)

// Sentinel errors, re-exported from the engine.
var (
	ErrConflict      = engine.ErrConflict
	ErrAlreadyActive = engine.ErrAlreadyActive
	ErrNotActive     = engine.ErrNotActive
)

// EPS key-changing procedures tracked for one UE. Each advances the {NH, NCC} AS
// key chain (TS 33.401 §7.2.8), so they are mutually exclusive and the registry
// keeps at most one active at a time. SecurityMode (the only core-initiated one) is
// attach-only, so it never collides with a RAN-initiated Path Switch / handover on a
// bearers-established UE; the only reachable collision is Handover⊗PathSwitch, where
// rejecting one preserves {NH, NCC} atomicity.
const (
	SecurityMode Type = "SecurityMode"
	S1Handover   Type = "S1Handover"
	PathSwitch   Type = "PathSwitch"
)

// NewRegistry returns a registry for the MME's key-chain procedures.
func NewRegistry(log *zap.Logger) *Registry {
	return engine.NewRegistry(log)
}
