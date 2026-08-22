// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/mattn/go-sqlite3"
)

var (
	ErrAlreadyExists            = errors.New("already exists")
	ErrNotFound                 = errors.New("not found")
	ErrDataNetworkNotFound      = errors.New("data network not found")
	ErrNoMatchingPolicy         = errors.New("no matching policy for slice and DNN")
	ErrDNNNotInSlice            = errors.New("data network not found in slice")
	ErrRestoreInProgress        = errors.New("a restore is already in progress")
	ErrInvalidBackupFile        = errors.New("uploaded file is not a valid SQLite database")
	ErrProposeTimeout           = errors.New("raft commit timeout")
	ErrOutcomeUnknown           = ellaraft.ErrOutcomeUnknown
	ErrMigrationPending         = errors.New("schema migration pending")
	ErrJoinTokenAlreadyConsumed = errors.New("join token already consumed")
	ErrUnknownOperation         = errors.New("unknown forwarded operation")
)

func isUniqueNameError(err error) bool {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return se.ExtendedCode == sqlite3.ErrConstraintUnique
	}

	return false
}
