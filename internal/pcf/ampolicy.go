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

func DeleteAMPolicy(ctx context.Context, supi string) error {
	_, span := tracer.Start(ctx, "PCF Delete AMPolicy")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", supi),
	)
	ue, err := pcfCtx.FindUEBySUPI(supi)
	if err != nil {
		return fmt.Errorf("ue not found in PCF for supi: %s", supi)
	}
	if ue == nil {
		return fmt.Errorf("ue not found in PCF for supi: %s", supi)
	}
	if ue.AMPolicyData == nil {
		return fmt.Errorf("policy association ID not found in PCF: %s", supi)
	}
	ue.AMPolicyData = nil
	return nil
}

func UpdateAMPolicy(ctx context.Context, supi string, policyAssociationUpdateRequest models.PolicyAssociationUpdateRequest) (*models.PolicyUpdate, error) {
	_, span := tracer.Start(ctx, "PCF Update AMPolicy")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", supi),
	)
	ue, err := pcfCtx.FindUEBySUPI(supi)
	if err != nil {
		return nil, fmt.Errorf("ue not found in PCF for supi: %s", supi)
	}
	if ue == nil || ue.AMPolicyData == nil {
		return nil, fmt.Errorf("supi not found in PCF")
	}

	amPolicyData := ue.AMPolicyData
	var response models.PolicyUpdate
	for _, trigger := range policyAssociationUpdateRequest.Triggers {
		switch trigger {
		case models.RequestTriggerLocCh:
			if policyAssociationUpdateRequest.UserLoc == nil {
				return nil, fmt.Errorf("UserLoc doesn't exist in Policy Association Requset Update while Triggers include LOC_CH")
			}
			amPolicyData.UserLoc = policyAssociationUpdateRequest.UserLoc
		default:
			return nil, fmt.Errorf("unknown request trigger: %s", trigger)
		}
	}

	response.Triggers = amPolicyData.Triggers

	return &response, nil
}

func CreateAMPolicy(ctx context.Context, policyAssociationRequest models.PolicyAssociationRequest) (*models.PolicyAssociation, error) {
	ctx, span := tracer.Start(ctx, "PCF Create AMPolicy")
	defer span.End()
	span.SetAttributes(
		attribute.String("supi", policyAssociationRequest.Supi),
	)

	var response models.PolicyAssociation
	var ue *UeContext
	if val, ok := pcfCtx.UePool.Load(policyAssociationRequest.Supi); ok {
		ue = val.(*UeContext)
	}
	if ue == nil {
		newUe, err := pcfCtx.NewPCFUe(policyAssociationRequest.Supi)
		if err != nil {
			return nil, fmt.Errorf("supi Format Error: %s", err.Error())
		}
		ue = newUe
	}
	response.Request = &policyAssociationRequest

	amPolicy := ue.AMPolicyData

	if amPolicy == nil {
		_, err := pcfCtx.DBInstance.GetSubscriber(ctx, ue.Supi)
		if err != nil {
			return nil, fmt.Errorf("ue not found in database: %s", ue.Supi)
		}
		amPolicy = ue.NewUeAMPolicyData(policyAssociationRequest)
	}

	if amPolicy.Rfsp != 0 {
		response.Rfsp = amPolicy.Rfsp
	}

	return &response, nil
}
