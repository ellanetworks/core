// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package consumer

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/omec-project/openapi/models"
)

func AMPolicyControlCreate(ue *context.AmfUe, anType models.AccessType) (*models.ProblemDetails, error) {
	amfSelf := context.AMF_Self()
	guamiList := context.GetServedGuamiList()

	policyAssociationRequest := models.PolicyAssociationRequest{
		NotificationUri: amfSelf.GetIPv4Uri() + "/namf-callback/v1/am-policy/",
		Supi:            ue.Supi,
		Pei:             ue.Pei,
		Gpsi:            ue.Gpsi,
		AccessType:      anType,
		ServingPlmn: &models.NetworkId{
			Mcc: ue.PlmnId.Mcc,
			Mnc: ue.PlmnId.Mnc,
		},
		Guami: &guamiList[0],
	}

	if ue.AccessAndMobilitySubscriptionData != nil {
		policyAssociationRequest.Rfsp = ue.AccessAndMobilitySubscriptionData.RfspIndex
	}

	res, locationHeader, err := pcf.CreateAMPolicy(policyAssociationRequest)
	if err != nil {
		logger.AmfLog.Warnf("Failed to create policy: %+v", err)
		problem := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return problem, err
	}
	ue.AmPolicyUri = locationHeader
	// re := regexp.MustCompile("/policies/.*")
	// match := re.FindStringSubmatch(locationHeader)
	ue.PolicyAssociationId = locationHeader
	ue.AmPolicyAssociation = res
	if res.Triggers != nil {
		for _, trigger := range res.Triggers {
			if trigger == models.RequestTrigger_LOC_CH {
				ue.RequestTriggerLocationChange = true
			}
		}
	}
	return nil, nil
}

func AMPolicyControlUpdate(ue *context.AmfUe, updateRequest models.PolicyAssociationUpdateRequest) (
	*models.ProblemDetails, error,
) {
	res, err := pcf.UpdateAMPolicy(ue.PolicyAssociationId, updateRequest)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return problemDetails, err
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
	return nil, nil
}

func AMPolicyControlDelete(ue *context.AmfUe) (*models.ProblemDetails, error) {
	err := pcf.DeleteAMPolicy(ue.PolicyAssociationId)
	if err != nil {
		problemDetails := &models.ProblemDetails{
			Status: 500,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return problemDetails, err
	}
	ue.RemoveAmPolicyAssociation()
	return nil, nil
}
