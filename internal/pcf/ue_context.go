// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"reflect"

	"github.com/ellanetworks/core/internal/models"
)

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
