// Copyright 2024 Ella Networks

package pcf

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

var pcfCtx *PCFContext

type PCFContext struct {
	UePool     sync.Map
	DBInstance *db.Database
}

type UeContext struct {
	Supi         string
	AMPolicyData *UeAMPolicyData
}

type UeAMPolicyData struct {
	AccessType  models.AccessType
	ServingPlmn *models.PlmnID
	UserLoc     *models.UserLocation
	Triggers    []models.RequestTrigger
	Rfsp        int32
}

func (ue *UeContext) NewUeAMPolicyData(req models.PolicyAssociationRequest) *UeAMPolicyData {
	ue.AMPolicyData = &UeAMPolicyData{
		AccessType:  req.AccessType,
		ServingPlmn: req.ServingPlmn,
		Rfsp:        req.Rfsp,
		UserLoc:     req.UserLoc,
	}
	return ue.AMPolicyData
}

// returns AM Policy which AccessType and plmnID match
func (ue *UeContext) FindAMPolicy(anType models.AccessType, plmnID *models.PlmnID) *UeAMPolicyData {
	if ue == nil || plmnID == nil || ue.AMPolicyData == nil {
		return nil
	}

	if ue.AMPolicyData.AccessType == anType && reflect.DeepEqual(*ue.AMPolicyData.ServingPlmn, *plmnID) {
		return ue.AMPolicyData
	}

	return nil
}

// Allocate PCF Ue with supi and add to pcf Context and returns allocated ue
func (c *PCFContext) NewUE(Supi string) (*UeContext, error) {
	newUeContext := &UeContext{}
	newUeContext.Supi = Supi
	c.UePool.Store(Supi, newUeContext)
	return newUeContext, nil
}

// Find PcfUe which the policyId belongs to
func (c *PCFContext) FindUEBySUPI(supi string) (*UeContext, error) {
	if value, ok := c.UePool.Load(supi); ok {
		return value.(*UeContext), nil
	}

	return nil, fmt.Errorf("ue not found in PCF for supi: %s", supi)
}

func Start(dbInstance *db.Database) error {
	pcfCtx = &PCFContext{
		DBInstance: dbInstance,
	}
	return nil
}
