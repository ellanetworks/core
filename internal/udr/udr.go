package udr

import (
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/udr/context"
)

func Start(dbInstance *db.Database) error {
	ctx := context.UDR_Self()
	ctx.DbInstance = dbInstance
	return nil
}
