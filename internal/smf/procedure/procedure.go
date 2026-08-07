// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the SMF's view of the shared procedure registry
// (internal/procedure): it defines the per-session procedure type set and binds
// it to the generic engine. The AMF and MME registries are per UE; this one is
// per session, because what it excludes are procedures that mutate one anchored
// session.
package procedure

import (
	engine "github.com/ellanetworks/core/internal/procedure"
	"go.uber.org/zap"
)

// Type and the registry are the generic engine's, re-exported so SMF code refers
// to a single procedure package.
type (
	Type     = engine.Type
	Registry = engine.Registry
)

var (
	ErrConflict      = engine.ErrConflict
	ErrAlreadyActive = engine.ErrAlreadyActive
	ErrNotActive     = engine.ErrNotActive
)

// Transfer is a move of the session between accesses (TS 23.502 §4.11.2). It
// spans several blocking calls to the UPF with the session lock dropped between
// them, so two of them interleaved would each re-point the same user plane and
// each tell an access to forget the session — leaving one no control plane owns.
const Transfer Type = "Transfer"

// NewRegistry returns a registry for one session's procedures.
func NewRegistry(log *zap.Logger) *Registry {
	return engine.NewRegistry(log)
}
