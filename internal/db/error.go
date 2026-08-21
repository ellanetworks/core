// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/mattn/go-sqlite3"
)

var (
	ErrAlreadyExists       = errors.New("already exists")
	ErrNotFound            = errors.New("not found")
	ErrDataNetworkNotFound = errors.New("data network not found")
	ErrNoMatchingPolicy    = errors.New("no matching policy for slice and DNN")
	// ErrDNNNotInSlice is returned by GetSessionPolicy when a policy matches the
	// requested slice but none serves the requested DNN.
	ErrDNNNotInSlice     = errors.New("data network not found in slice")
	ErrRestoreInProgress = errors.New("a restore is already in progress")
	ErrInvalidBackupFile = errors.New("uploaded file is not a valid SQLite database")
	// ErrProposeTimeout is returned when a Raft proposal was rejected before
	// dispatch (queue full, not leader, or Raft shutting down). Nothing was
	// applied, so callers may retry; the API maps it to 503.
	ErrProposeTimeout = errors.New("raft commit timeout")

	// ErrOutcomeUnknown reports a write that may or may not have been
	// applied. Callers must verify state rather than retry.
	ErrOutcomeUnknown = ellaraft.ErrOutcomeUnknown
	// ErrMigrationPending is returned when a handler depends on a schema
	// version the cluster has not yet rolled forward to. Surfaces as 503
	// with Retry-After so clients back off until the slowest voter
	// catches up and the leader proposes the migration.
	ErrMigrationPending = errors.New("schema migration pending")
	// ErrJoinTokenAlreadyConsumed is returned by ConsumeJoinToken when the
	// conditional UPDATE affected zero rows — either the id is unknown or
	// the token has already been consumed by a prior (racing) caller.
	ErrJoinTokenAlreadyConsumed = errors.New("join token already consumed")
	// ErrUnknownOperation is returned by ApplyForwardedOperation when the
	// operation name is not in the registered dispatch table. The HTTP
	// handler maps it to 400 so a buggy follower surfaces as a client
	// error rather than fail-stopping the leader.
	ErrUnknownOperation = errors.New("unknown forwarded operation")
)

func isUniqueNameError(err error) bool {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return se.ExtendedCode == sqlite3.ErrConstraintUnique
	}

	return false
}
