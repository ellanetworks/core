// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the AMF's view of the shared procedure registry
// (internal/procedure): it defines the 5GMM key-changing procedure type set and the
// TS 33.501 §6.9.5 mutual-exclusion rule, and binds them to the generic engine.
package procedure

import (
	engine "github.com/ellanetworks/core/internal/procedure"
	"go.uber.org/zap"
)

// Type and the registry primitives are the generic engine's, re-exported so AMF
// code refers to a single procedure package.
type (
	Type      = engine.Type
	Procedure = engine.Procedure
	ID        = engine.ID
	Registry  = engine.Registry
)

var (
	ErrConflict      = engine.ErrConflict
	ErrAlreadyActive = engine.ErrAlreadyActive
	ErrNotActive     = engine.ErrNotActive
)

// The key-changing procedures tracked for one UE. All of them mutate the UE's single
// {NH, NCC} / KgNB key chain — SecurityMode activates a new K_AMF the chain derives
// from, and N2Handover and PathSwitch each increment NCC and compute a fresh NH from
// the currently active K_AMF (TS 33.501 §6.9.2.3.2/§6.9.2.3.3) — so per §6.9.5 they are
// mutually exclusive. Authentication is deliberately NOT tracked: its new K_AMF stays
// inactive until a SecurityMode takes it into use (§6.9.4.2), so it may overlap them.
const (
	SecurityMode Type = "SecurityMode"
	N2Handover   Type = "N2Handover"
	PathSwitch   Type = "PathSwitch"
)

// NewRegistry returns a registry bound to the AMF's mutual-exclusion rule.
func NewRegistry(log *zap.Logger) *Registry {
	return engine.NewRegistry(log, keyChainRules{})
}

// keyChainRules makes the tracked key-changing procedures mutually exclusive: any active one
// blocks any other, since they all mutate the one {NH, NCC}/KgNB chain (TS 33.501
// §6.9.5).
//
// A conflict rejects the incoming procedure. That is compliant because SecurityMode (the
// only core-initiated one) is registration-only, so it never collides with a RAN-initiated
// Path Switch / handover on a bearers-established UE; the only reachable collision is
// Handover⊗PathSwitch, where rejecting one preserves {NH,NCC} atomicity. If mid-session
// re-keying is ever added, SMC-vs-RAN-initiated must instead proceed with the OLD key
// (TS 33.501 §6.9.2.3.2 NSCI), not reject.
type keyChainRules struct{}

func (keyChainRules) Conflicts(active, incoming Type) (bool, string) {
	return true, "key-chain"
}

// Reentrant is false for every tracked type: at most one instance of each may be active.
func (keyChainRules) Reentrant(Type) bool { return false }
