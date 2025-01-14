// Copyright 2024 Ella Networks

package udm

import (
	"math"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/ellanetworks/core/internal/util/suci"
	"github.com/omec-project/openapi/models"
)

const (
	UDM_HNP_PRIVATE_KEY = "c09c17bddf23357f614f492075b970d825767718114f59554ce2f345cf8c4b6a"
)

func Start(dbInstance *db.Database) error {
	udmContext.UriScheme = models.UriScheme_HTTP
	udmContext.SuciProfiles = []suci.SuciProfile{
		{
			ProtectionScheme: "1", // Standard defined value for Protection Scheme A (TS 33.501 Annex C)
			PrivateKey:       UDM_HNP_PRIVATE_KEY,
		},
	}
	udmContext.NfService = make(map[models.ServiceName]models.NfService)
	udmContext.EeSubscriptionIDGenerator = idgenerator.NewGenerator(1, math.MaxInt32)
	udmContext.SdmSubscriptionIDGenerator = 1
	udmContext.DbInstance = dbInstance
	return nil
}
