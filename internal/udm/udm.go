// Copyright 2024 Ella Networks

package udm

import (
	"github.com/ellanetworks/core/internal/db"
)

func Start(dbInstance *db.Database) error {
	udmContext.DBInstance = dbInstance
	return nil
}
