package context

import (
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/omec-project/openapi/models"
)

var udrContext = UDRContext{}

type subsId = string

type UDRServiceType int

const (
	NUDR_DR UDRServiceType = iota
)

func init() {
	UDR_Self().EeSubscriptionIDGenerator = 1
	UDR_Self().SdmSubscriptionIDGenerator = 1
	UDR_Self().SubscriptionDataSubscriptionIDGenerator = 1
	UDR_Self().PolicyDataSubscriptionIDGenerator = 1
	UDR_Self().SubscriptionDataSubscriptions = make(map[subsId]*models.SubscriptionDataSubscriptions)
	UDR_Self().PolicyDataSubscriptions = make(map[subsId]*models.PolicyDataSubscription)
}

type UDRContext struct {
	SubscriptionDataSubscriptions           map[subsId]*models.SubscriptionDataSubscriptions
	PolicyDataSubscriptions                 map[subsId]*models.PolicyDataSubscription
	UESubsCollection                        sync.Map // map[ueId]*UESubsData
	UEGroupCollection                       sync.Map // map[ueGroupId]*UEGroupSubsData
	EeSubscriptionIDGenerator               int
	SdmSubscriptionIDGenerator              int
	PolicyDataSubscriptionIDGenerator       int
	SubscriptionDataSubscriptionIDGenerator int
	DbInstance                              *db.Database
}

type UESubsData struct {
	EeSubscriptionCollection map[subsId]*EeSubscriptionCollection
	SdmSubscriptions         map[subsId]*models.SdmSubscription
}

type UEGroupSubsData struct {
	EeSubscriptions map[subsId]*models.EeSubscription
}

type EeSubscriptionCollection struct {
	EeSubscriptions      *models.EeSubscription
	AmfSubscriptionInfos []models.AmfSubscriptionInfo
}

// Create new UDR context
func UDR_Self() *UDRContext {
	return &udrContext
}
