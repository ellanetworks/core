// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the SMF's view of the shared procedure registry
// (internal/procedure): it defines the session-management procedure type set and
// binds it to the generic engine.
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

// The procedures tracked for one session. They are mutually exclusive: each
// rewrites the session across several blocking UPF calls.
const (
	// Transfer moves a session to the other access (TS 23.502 §4.11.2).
	Transfer Type = "Transfer"
)

func NewRegistry(log *zap.Logger) *Registry { return engine.NewRegistry(log) }
