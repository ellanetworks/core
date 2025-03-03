// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"fmt"
	"time"

	amf_context "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Namf_Communication"
	"github.com/omec-project/openapi/models"
)

func BuildUeContextModel(ue *amf_context.AmfUe) (ueContext coreModels.UeContext) {
	ueContext.Supi = ue.Supi
	ueContext.SupiUnauthInd = ue.UnauthenticatedSupi

	if ue.Gpsi != "" {
		ueContext.GpsiList = append(ueContext.GpsiList, ue.Gpsi)
	}

	if ue.Pei != "" {
		ueContext.Pei = ue.Pei
	}

	if ue.UdmGroupId != "" {
		ueContext.UdmGroupId = ue.UdmGroupId
	}

	if ue.AusfGroupId != "" {
		ueContext.AusfGroupId = ue.AusfGroupId
	}

	if ue.RoutingIndicator != "" {
		ueContext.RoutingIndicator = ue.RoutingIndicator
	}

	if ue.AccessAndMobilitySubscriptionData != nil {
		if ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr != nil {
			ueContext.SubUeAmbr = &coreModels.Ambr{
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

func buildAmPolicyReqTriggers(triggers []coreModels.RequestTrigger) (amPolicyReqTriggers []coreModels.AmPolicyReqTrigger) {
	for _, trigger := range triggers {
		switch trigger {
		case coreModels.RequestTrigger_LOC_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, coreModels.AmPolicyReqTrigger_LOCATION_CHANGE)
		case coreModels.RequestTrigger_PRA_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, coreModels.AmPolicyReqTrigger_PRA_CHANGE)
		case coreModels.RequestTrigger_SERV_AREA_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, coreModels.AmPolicyReqTrigger_SARI_CHANGE)
		case coreModels.RequestTrigger_RFSP_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, coreModels.AmPolicyReqTrigger_RFSP_INDEX_CHANGE)
		}
	}
	return
}

func UEContextTransferRequest(
	ue *amf_context.AmfUe, accessType coreModels.AccessType, transferReason models.TransferReason) (
	ueContextTransferRspData *coreModels.UeContextTransferRspData, problemDetails *models.ProblemDetails, err error,
) {
	logger.AmfLog.Warnf("UE context transfer request is not implemented")
	return nil, nil, nil
}

// This operation is called "RegistrationCompleteNotify" at TS 23.502
func RegistrationStatusUpdate(ue *amf_context.AmfUe, request models.UeRegStatusUpdateReqData) (
	regStatusTransferComplete bool, problemDetails *models.ProblemDetails, err error,
) {
	configuration := Namf_Communication.NewConfiguration()
	configuration.SetBasePath(ue.TargetAmfUri)
	client := Namf_Communication.NewAPIClient(configuration)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ueContextId := fmt.Sprintf("5g-guti-%s", ue.Guti)
	res, httpResp, localErr := client.IndividualUeContextDocumentApi.RegistrationStatusUpdate(ctx, ueContextId, request)
	if localErr == nil {
		regStatusTransferComplete = res.RegStatusTransferComplete
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			err = localErr
			return
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("%s: server no response", ue.TargetAmfUri)
	}
	return
}
