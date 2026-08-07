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

// The procedures tracked for one session. Each rewrites the session while
// holding its lock across blocking UPF calls, so a second one left queued behind
// the first commits against state the first has since changed. The registry
// refuses it. No clause requires refusing over serialising the two, so the
// refusal is a local choice.
const (
	// Release tears the session down (TS 23.502 §4.3.4).
	Release Type = "Release"
)

func NewRegistry(log *zap.Logger) *Registry { return engine.NewRegistry(log) }
