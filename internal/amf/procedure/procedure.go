// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the AMF's view of the shared procedure registry
// (internal/procedure): it defines the 5GMM key-changing procedure type set and
// binds it to the generic engine.
package procedure

import (
	engine "github.com/ellanetworks/core/internal/procedure"
	"go.uber.org/zap"
)

// Type and the registry are the generic engine's, re-exported so AMF code refers to
// a single procedure package.
type (
	Type     = engine.Type
	Registry = engine.Registry
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
// mutually exclusive, and the registry keeps at most one active at a time.
// Authentication is deliberately NOT tracked: its new K_AMF stays inactive until a
// SecurityMode takes it into use (§6.9.4.2), so it may overlap them.
const (
	SecurityMode Type = "SecurityMode"
	N2Handover   Type = "N2Handover"
	PathSwitch   Type = "PathSwitch"
)

// NewRegistry returns a registry for the AMF's key-chain procedures.
func NewRegistry(log *zap.Logger) *Registry {
	return engine.NewRegistry(log)
}
