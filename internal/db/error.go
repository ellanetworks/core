package db

import (
	"errors"

	"github.com/mattn/go-sqlite3"
)

var ErrAlreadyExists = errors.New("already exists")

func isUniqueNameError(err error) bool {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return se.ExtendedCode == sqlite3.ErrConstraintUnique
	}

	return false
}
