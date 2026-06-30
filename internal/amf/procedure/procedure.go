// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the AMF's view of the shared procedure registry
// (internal/procedure): it defines the 5GMM procedure type set and the TS 33.501
// §6.9.5.1 conflict matrix (matrix.go), and binds them to the generic engine.
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

// Sentinel errors, re-exported from the engine.
var (
	ErrConflict      = engine.ErrConflict
	ErrAlreadyActive = engine.ErrAlreadyActive
	ErrNotActive     = engine.ErrNotActive
)

// 5GMM procedure types tracked for one UE.
const (
	Registration   Type = "Registration"
	Authentication Type = "Authentication"
	SecurityMode   Type = "SecurityMode"
	N2Handover     Type = "N2Handover"
	UEContextMod   Type = "UEContextModification"
	Paging         Type = "Paging"
)

// NewRegistry returns a registry bound to the AMF conflict matrix.
func NewRegistry(log *zap.Logger) *Registry {
	return engine.NewRegistry(log, amfRules{})
}
