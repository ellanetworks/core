package udr

import (
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udr/context"
)

func Start(dbInstance *db.Database) error {
	ctx := context.UDR_Self()
	ctx.DbInstance = dbInstance
	return nil
}
