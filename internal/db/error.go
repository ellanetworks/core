package db

import (
	"errors"

	"github.com/mattn/go-sqlite3"
)

var (
	ErrAlreadyExists     = errors.New("already exists")
	ErrNotFound          = errors.New("not found")
	ErrRestoreInProgress = errors.New("a restore is already in progress")
	ErrInvalidBackupFile = errors.New("uploaded file is not a valid SQLite database")
)

func isUniqueNameError(err error) bool {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return se.ExtendedCode == sqlite3.ErrConstraintUnique
	}

	return false
}
