// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package consumer

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
)

func AMPolicyControlCreate(ue *context.AmfUe, anType models.AccessType) error {
	guamiList := context.GetServedGuamiList()

	policyAssociationRequest := models.PolicyAssociationRequest{
		Supi:       ue.Supi,
		Pei:        ue.Pei,
		Gpsi:       ue.Gpsi,
		AccessType: anType,
		ServingPlmn: &models.PlmnID{
			Mcc: ue.PlmnID.Mcc,
			Mnc: ue.PlmnID.Mnc,
		},
		Guami: &models.Guami{
			PlmnID: &models.PlmnID{
				Mcc: guamiList[0].PlmnID.Mcc,
				Mnc: guamiList[0].PlmnID.Mnc,
			},
			AmfID: guamiList[0].AmfID,
		},
	}

	if ue.AccessAndMobilitySubscriptionData != nil {
		policyAssociationRequest.Rfsp = ue.AccessAndMobilitySubscriptionData.RfspIndex
	}

	res, locationHeader, err := pcf.CreateAMPolicy(policyAssociationRequest)
	if err != nil {
		return fmt.Errorf("failed to create policy: %+v", err)
	}
	ue.PolicyAssociationId = locationHeader
	ue.AmPolicyAssociation = res
	if res.Triggers != nil {
		for _, trigger := range res.Triggers {
			if trigger == models.RequestTrigger_LOC_CH {
				ue.RequestTriggerLocationChange = true
			}
		}
	}
	return nil
}

func AMPolicyControlUpdate(ue *context.AmfUe, updateRequest models.PolicyAssociationUpdateRequest) error {
	res, err := pcf.UpdateAMPolicy(ue.PolicyAssociationId, updateRequest)
	if err != nil {
		return fmt.Errorf("failed to update policy: %+v", err)
	}
	if res.ServAreaRes != nil {
		ue.AmPolicyAssociation.ServAreaRes = res.ServAreaRes
	}
	if res.Rfsp != 0 {
		ue.AmPolicyAssociation.Rfsp = res.Rfsp
	}
	ue.AmPolicyAssociation.Triggers = res.Triggers
	ue.RequestTriggerLocationChange = false
	for _, trigger := range res.Triggers {
		if trigger == models.RequestTrigger_LOC_CH {
			ue.RequestTriggerLocationChange = true
		}
	}
	return nil
}

func AMPolicyControlDelete(ue *context.AmfUe) error {
	err := pcf.DeleteAMPolicy(ue.PolicyAssociationId)
	if err != nil {
		return fmt.Errorf("could not delete policy: %+v", err)
	}
	ue.RemoveAmPolicyAssociation()
	return nil
}
