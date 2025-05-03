// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("ella-core/pcf")

func DeleteAMPolicy(polAssoID string, ctx context.Context) error {
	_, span := tracer.Start(ctx, "DeleteAMPolicy")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.policy_association_id", polAssoID),
	)
	ue, err := pcfCtx.PCFUeFindByPolicyID(polAssoID)
	if err != nil {
		return fmt.Errorf("ue not found in PCF for policy association ID: %s", polAssoID)
	}
	if ue == nil {
		return fmt.Errorf("ue not found in PCF for policy association ID: %s", polAssoID)
	}
	_, exists := ue.AMPolicyData[polAssoID]
	if !exists {
		return fmt.Errorf("policy association ID not found in PCF: %s", polAssoID)
	}
	delete(ue.AMPolicyData, polAssoID)
	return nil
}

func UpdateAMPolicy(polAssoID string, policyAssociationUpdateRequest models.PolicyAssociationUpdateRequest, ctx context.Context) (*models.PolicyUpdate, error) {
	_, span := tracer.Start(ctx, "UpdateAMPolicy")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.policy_association_id", polAssoID),
	)
	ue, err := pcfCtx.PCFUeFindByPolicyID(polAssoID)
	if err != nil {
		return nil, fmt.Errorf("ue not found in PCF for policy association ID: %s", polAssoID)
	}
	if ue == nil || ue.AMPolicyData[polAssoID] == nil {
		return nil, fmt.Errorf("polAssoID not found  in PCF")
	}

	amPolicyData := ue.AMPolicyData[polAssoID]
	var response models.PolicyUpdate
	for _, trigger := range policyAssociationUpdateRequest.Triggers {
		switch trigger {
		case models.RequestTriggerLocCh:
			if policyAssociationUpdateRequest.UserLoc == nil {
				return nil, fmt.Errorf("UserLoc doesn't exist in Policy Association Requset Update while Triggers include LOC_CH")
			}
			amPolicyData.UserLoc = policyAssociationUpdateRequest.UserLoc
		case models.RequestTriggerPraCh:
			if policyAssociationUpdateRequest.PraStatuses == nil {
				return nil, fmt.Errorf("PraStatuses doesn't exist in Policy Association")
			}
		case models.RequestTriggerServAreaCh:
			if policyAssociationUpdateRequest.ServAreaRes == nil {
				return nil, fmt.Errorf("ServAreaRes doesn't exist in Policy Association Requset Update while Triggers include SERV_AREA_CH")
			} else {
				amPolicyData.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
				response.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
			}
		case models.RequestTriggerRfspCh:
			if policyAssociationUpdateRequest.Rfsp == 0 {
				return nil, fmt.Errorf("rfsp doesn't exist in Policy Association Requset Update while Triggers include RFSP_CH")
			} else {
				amPolicyData.Rfsp = policyAssociationUpdateRequest.Rfsp
				response.Rfsp = policyAssociationUpdateRequest.Rfsp
			}
		}
	}

	response.Triggers = amPolicyData.Triggers

	return &response, nil
}

func CreateAMPolicy(policyAssociationRequest models.PolicyAssociationRequest, ctx context.Context) (*models.PolicyAssociation, string, error) {
	ctx, span := tracer.Start(ctx, "CreateAMPolicy")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", policyAssociationRequest.Supi),
	)
	var response models.PolicyAssociation
	var ue *UeContext
	if val, ok := pcfCtx.UePool.Load(policyAssociationRequest.Supi); ok {
		ue = val.(*UeContext)
	}
	if ue == nil {
		newUe, err := pcfCtx.NewPCFUe(policyAssociationRequest.Supi)
		if err != nil {
			return nil, "", fmt.Errorf("supi Format Error: %s", err.Error())
		}
		ue = newUe
	}
	response.Request = &policyAssociationRequest
	assolID := fmt.Sprintf("%s-%d", ue.Supi, ue.PolAssociationIDGenerator)
	amPolicy := ue.AMPolicyData[assolID]

	if amPolicy == nil {
		_, err := pcfCtx.DBInstance.GetSubscriber(ue.Supi, ctx)
		if err != nil {
			return nil, "", fmt.Errorf("ue not found in database: %s", ue.Supi)
		}
		amPolicy = ue.NewUeAMPolicyData(assolID, policyAssociationRequest)
	}

	if amPolicy.Rfsp != 0 {
		response.Rfsp = amPolicy.Rfsp
	}
	ue.PolAssociationIDGenerator++
	return &response, assolID, nil
}
