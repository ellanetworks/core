// Copyright 2024 Ella Networks

package udr

import (
	"github.com/ellanetworks/core/internal/db"
)

func Start(dbInstance *db.Database) error {
	NewUdrContext(1, dbInstance)
	return nil
}
