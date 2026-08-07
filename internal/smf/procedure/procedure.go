// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package procedure

import (
	engine "github.com/ellanetworks/core/internal/procedure"
	"go.uber.org/zap"
)

type (
	Type     = engine.Type
	Registry = engine.Registry
)

var (
	ErrConflict      = engine.ErrConflict
	ErrAlreadyActive = engine.ErrAlreadyActive
	ErrNotActive     = engine.ErrNotActive
)

const Transfer Type = "Transfer"

func NewRegistry(log *zap.Logger) *Registry {
	return engine.NewRegistry(log)
}
