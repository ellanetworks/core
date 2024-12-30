package udr

import (
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/omec-project/openapi/models"
)

var udrContext = UDRContext{}

type subsId = string

type UDRServiceType int

type UDRContext struct {
	UESubsCollection           sync.Map // map[ueId]*UESubsData
	SdmSubscriptionIDGenerator int
	DbInstance                 *db.Database
}

type UESubsData struct {
	EeSubscriptionCollection map[subsId]*EeSubscriptionCollection
	SdmSubscriptions         map[subsId]*models.SdmSubscription
}

type EeSubscriptionCollection struct {
	EeSubscriptions      *models.EeSubscription
	AmfSubscriptionInfos []models.AmfSubscriptionInfo
}

func Start(dbInstance *db.Database) error {
	udrContext.SdmSubscriptionIDGenerator = 1
	udrContext.DbInstance = dbInstance
	return nil
}
