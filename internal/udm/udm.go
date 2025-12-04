// Copyright 2024 Ella Networks

package udm

import (
	"github.com/ellanetworks/core/internal/db"
)

var udmContext UDMContext

type UDMContext struct {
	DBInstance *db.Database
}

func SetDBInstance(dbInstance *db.Database) {
	udmContext.DBInstance = dbInstance
}

func Start(dbInstance *db.Database) error {
	udmContext.DBInstance = dbInstance
	return nil
}
