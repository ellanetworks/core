package udr

import (
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/omec-project/openapi/models"
)

var udrContext = UDRContext{}

type subsId = string

type UDRContext struct {
	UESubsCollection           sync.Map // map[ueId]*UESubsData
	SdmSubscriptionIDGenerator int
	DbInstance                 Database
}

type UESubsData struct {
	SdmSubscriptions map[subsId]*models.SdmSubscription
}

type Database interface {
	GetSubscriber(ueId string) (*db.Subscriber, error)
	GetProfileByID(profileID int) (*db.Profile, error)
	UpdateSubscriber(subscriber *db.Subscriber) error
}

func NewUdrContext(idGenerator int, dbInstance Database) *UDRContext {
	udrContext = UDRContext{
		SdmSubscriptionIDGenerator: idGenerator,
		DbInstance:                 dbInstance,
	}
	return &udrContext
}
