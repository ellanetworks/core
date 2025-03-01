// Copyright 2024 Ella Networks

package udm

import (
	"github.com/ellanetworks/core/internal/db"
)

func Start(dbInstance *db.Database) error {
	udmContext.SdmSubscriptionIDGenerator = 1
	udmContext.DbInstance = dbInstance
	return nil
}
