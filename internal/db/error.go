package db

import (
	"errors"

	"github.com/mattn/go-sqlite3"
)

var (
	ErrAlreadyExists       = errors.New("already exists")
	ErrNotFound            = errors.New("not found")
	ErrDataNetworkNotFound = errors.New("data network not found")
	ErrRestoreInProgress   = errors.New("a restore is already in progress")
	ErrInvalidBackupFile   = errors.New("uploaded file is not a valid SQLite database")
	// ErrProposeTimeout is returned when a Raft proposal cannot be committed
	// (queue full, leader lost mid-commit, or Raft shutting down). Callers
	// should treat it as a transient 503 condition.
	ErrProposeTimeout = errors.New("raft commit timeout")
)

func isUniqueNameError(err error) bool {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return se.ExtendedCode == sqlite3.ErrConstraintUnique
	}

	return false
}
