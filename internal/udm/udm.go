// Copyright 2024 Ella Networks

package udm

import (
	"github.com/ellanetworks/core/internal/db"
	"github.com/omec-project/openapi/models"
)

func Start(dbInstance *db.Database) error {
	udmContext.UriScheme = models.UriScheme_HTTP
	udmContext.NfService = make(map[models.ServiceName]models.NfService)
	udmContext.SdmSubscriptionIDGenerator = 1
	udmContext.DbInstance = dbInstance
	return nil
}
