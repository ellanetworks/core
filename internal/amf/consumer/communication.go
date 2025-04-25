// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func BuildUeContextModel(ue *context.AmfUe) (ueContext models.UeContext) {
	ueContext.Supi = ue.Supi

	if ue.Gpsi != "" {
		ueContext.GpsiList = append(ueContext.GpsiList, ue.Gpsi)
	}

	if ue.Pei != "" {
		ueContext.Pei = ue.Pei
	}

	if ue.RoutingIndicator != "" {
		ueContext.RoutingIndicator = ue.RoutingIndicator
	}

	if ue.AccessAndMobilitySubscriptionData != nil {
		if ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr != nil {
			ueContext.SubUeAmbr = &models.Ambr{
				Uplink:   ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr.Uplink,
				Downlink: ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr.Downlink,
			}
		}
		if ue.AccessAndMobilitySubscriptionData.RfspIndex != 0 {
			ueContext.SubRfsp = ue.AccessAndMobilitySubscriptionData.RfspIndex
		}
	}

	if ue.AmPolicyAssociation != nil {
		if len(ue.AmPolicyAssociation.Triggers) > 0 {
			ueContext.AmPolicyReqTriggerList = buildAmPolicyReqTriggers(ue.AmPolicyAssociation.Triggers)
		}
	}

	if ue.TraceData != nil {
		ueContext.TraceData = ue.TraceData
	}
	return ueContext
}

func buildAmPolicyReqTriggers(triggers []models.RequestTrigger) (amPolicyReqTriggers []models.AmPolicyReqTrigger) {
	for _, trigger := range triggers {
		switch trigger {
		case models.RequestTriggerLocCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerLocationChange)
		case models.RequestTriggerPraCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerPraChange)
		case models.RequestTriggerServAreaCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerSariChange)
		case models.RequestTriggerRfspCh:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTriggerRfspIndexChange)
		}
	}
	return
}

func UEContextTransferRequest(
	ue *context.AmfUe, accessType models.AccessType, transferReason models.TransferReason) (
	*models.UeContextTransferRspData, error,
) {
	logger.AmfLog.Warn("UE context transfer request is not implemented")
	return nil, nil
}

// This operation is called "RegistrationCompleteNotify" at TS 23.502
func RegistrationStatusUpdate(ue *context.AmfUe, request models.UeRegStatusUpdateReqData) (
	bool, error,
) {
	logger.AmfLog.Warn("UE registration status update is not implemented")
	return false, nil
}
