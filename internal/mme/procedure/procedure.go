// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure defines the MME's EPS key-changing procedure type set and
// their mutual-exclusion rules, binding them to the shared procedure engine
// (internal/procedure).
package procedure

import (
	engine "github.com/ellanetworks/core/internal/procedure"
	"go.uber.org/zap"
)

// Type and the registry primitives are re-exported from the engine so MME code
// refers to a single procedure package.
type (
	Type      = engine.Type
	Procedure = engine.Procedure
	ID        = engine.ID
	Registry  = engine.Registry
)

// Sentinel errors, re-exported from the engine.
var (
	ErrConflict      = engine.ErrConflict
	ErrAlreadyActive = engine.ErrAlreadyActive
	ErrNotActive     = engine.ErrNotActive
)

// EPS key-changing procedures tracked for one UE. Each advances the {NH, NCC} AS
// key chain (TS 33.401 §7.2.8), so they are mutually exclusive.
const (
	SecurityMode Type = "SecurityMode"
	S1Handover   Type = "S1Handover"
	PathSwitch   Type = "PathSwitch"
)

// NewRegistry returns a registry bound to the MME key-chain mutual-exclusion rules.
func NewRegistry(log *zap.Logger) *Registry {
	return engine.NewRegistry(log, keyChainRules{})
}

// keyChainRules makes every key-changing procedure mutually exclusive: none may run
// while another advances the {NH, NCC} chain (TS 33.401 §7.2.8). The coarse rule —
// any one blocks the others — matches the MME's single per-UE key chain.
//
// A conflict rejects the incoming procedure. That is compliant because SecurityMode (the
// only core-initiated one) is attach-only, so it never collides with a RAN-initiated
// Path Switch / handover on a bearers-established UE; the only reachable collision is
// Handover⊗PathSwitch, where rejecting one preserves {NH,NCC} atomicity.
type keyChainRules struct{}

// Conflicts blocks any incoming key-changing procedure while another is active.
// The engine calls it only for distinct types, so returning true unconditionally
// makes all three mutually exclusive.
func (keyChainRules) Conflicts(active, incoming Type) (bool, string) {
	return true, "key-chain"
}

// Reentrant is false for all types: at most one instance of each may be active.
func (keyChainRules) Reentrant(Type) bool { return false }
