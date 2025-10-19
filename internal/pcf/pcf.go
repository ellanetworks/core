// Copyright 2024 Ella Networks

package pcf

import (
	"github.com/ellanetworks/core/internal/db"
)

func Start(dbInstance *db.Database) error {
	pcfCtx = &PCFContext{
		DBInstance: dbInstance,
	}
	return nil
}
