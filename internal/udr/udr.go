// Copyright 2024 Ella Networks

package udr

import (
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/omec-project/openapi/models"
)

var udrContext = UDRContext{}

type subsID = string

type UDRServiceType int

type UDRContext struct {
	UESubsCollection           sync.Map // map[ueId]*UESubsData
	SdmSubscriptionIDGenerator int
	DBInstance                 *db.Database
}

type UESubsData struct {
	EeSubscriptionCollection map[subsID]*EeSubscriptionCollection
	SdmSubscriptions         map[subsID]*models.SdmSubscription
}

type EeSubscriptionCollection struct {
	EeSubscriptions      *models.EeSubscription
	AmfSubscriptionInfos []models.AmfSubscriptionInfo
}

func Start(dbInstance *db.Database) error {
	udrContext.SdmSubscriptionIDGenerator = 1
	udrContext.DBInstance = dbInstance
	return nil
}
