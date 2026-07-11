// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"

	"github.com/mattn/go-sqlite3"
)

var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
	// ErrLeaseActive is returned when a repin or delete targets a static
	// reservation bound to a session (op guard: WHERE sessionID IS NULL).
	// Handlers map it to 409.
	ErrLeaseActive         = errors.New("static IP is in use by an active session")
	ErrDataNetworkNotFound = errors.New("data network not found")
	ErrNoMatchingPolicy    = errors.New("no matching policy for slice and DNN")
	// ErrDNNNotInSlice is returned by GetSessionPolicy when a policy matches the
	// requested slice but none serves the requested DNN.
	ErrDNNNotInSlice     = errors.New("data network not found in slice")
	ErrRestoreInProgress = errors.New("a restore is already in progress")
	ErrInvalidBackupFile = errors.New("uploaded file is not a valid SQLite database")
	// ErrProposeTimeout is returned when a Raft proposal cannot be committed
	// (queue full, leader lost mid-commit, or Raft shutting down). Callers
	// should treat it as a transient 503 condition.
	ErrProposeTimeout = errors.New("raft commit timeout")
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
