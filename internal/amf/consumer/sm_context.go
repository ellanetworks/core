// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
)

func SelectSmf(
	anType models.AccessType,
	pduSessionID int32,
	snssai models.Snssai,
	dnn string,
) *context.SmContext {
	smContext := context.NewSmContext(pduSessionID)

	smContext.SetSnssai(snssai)
	smContext.SetDnn(dnn)
	smContext.SetAccessType(anType)

	return smContext
}

func SendCreateSmContextRequest(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, nasPdu []byte) (string, *models.PostSmContextsErrorResponse, error) {
	snssai := smContext.Snssai()

	postSmContextsRequest := models.PostSmContextsRequest{
		JSONData: &models.SmContextCreateData{
			Supi:         ue.Supi,
			PduSessionID: smContext.PduSessionID(),
			SNssai: &models.Snssai{
				Sst: snssai.Sst,
				Sd:  snssai.Sd,
			},
			Dnn: smContext.Dnn(),
		},
		BinaryDataN1SmMessage: nasPdu,
	}

	return pdusession.CreateSmContext(ctx, postSmContextsRequest)
}

// Upadate SmContext Request
// servingNfID, smContextStatusUri, guami, servingNetwork -> amf change
// anType -> anType change
// ratType -> ratType change
// presenceInLadn -> Service Request , Xn handover, N2 handover and dnn is a ladn
// ueLocation -> the user location has changed or the user plane of the PDU session is deactivated
// upCnxState -> request the activation or the deactivation of the user plane connection of the PDU session
// hoState -> the preparation, execution or cancellation of a handover of the PDU session
// toBeSwitch -> Xn Handover to request to switch the PDU session to a new downlink N3 tunnel endpoint
// failedToBeSwitch -> indicate that the PDU session failed to be setup in the target RAN
// targetID, targetServingNfID(preparation with AMF change) -> N2 handover
// release -> duplicated PDU Session Id in subclause 5.2.2.3.11, slice not available in subclause 5.2.2.3.12
// ngApCause -> e.g. the NGAP cause for requesting to deactivate the user plane connection of the PDU session.
// 5gMmCauseValue -> AMF received a 5GMM cause code from the UE e.g 5GMM Status message in response to
// a Downlink NAS Transport message carrying 5GSM payload
// anTypeCanBeChanged

func SendUpdateSmContextActivateUpCnxState(
	ctx ctxt.Context,
	ue *context.AmfUe, smContext *context.SmContext, accessType models.AccessType) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxStateActivating

	if smContext.AccessType() != accessType {
		updateData.AnType = smContext.AccessType()
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextDeactivateUpCnxState(ctx ctxt.Context, ue *context.AmfUe,
	smContext *context.SmContext, cause context.CauseAll) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxStateDeactivated
	if cause.Cause != nil {
		updateData.Cause = *cause.Cause
	}
	if cause.NgapCause != nil {
		updateData.NgApCause = &models.NgApCause{
			Group: cause.NgapCause.Group,
			Value: cause.NgapCause.Value,
		}
	}
	if cause.Var5GmmCause != nil {
		updateData.Var5gMmCauseValue = *cause.Var5GmmCause
	}
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextChangeAccessType(ctx ctxt.Context, ue *context.AmfUe,
	smContext *context.SmContext, anTypeCanBeChanged bool) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.AnTypeCanBeChanged = anTypeCanBeChanged
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2Info(
	ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.N2SmInfoType = n2SmType
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandover(
	ctx ctxt.Context,
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
	}
	updateData.ToBeSwitched = true

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandoverFailed(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
	}
	updateData.FailedToBeSwitched = true
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverPreparing(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte, amfid string, targetID *models.NgRanTargetID) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
	}
	updateData.HoState = models.HoStatePreparing
	updateData.TargetID = &models.NgRanTargetID{
		RanNodeID: &models.GlobalRanNodeID{
			PlmnID: &models.PlmnID{
				Mcc: targetID.RanNodeID.PlmnID.Mcc,
				Mnc: targetID.RanNodeID.PlmnID.Mnc,
			},
			GNbID: &models.GNbID{
				BitLength: targetID.RanNodeID.GNbID.BitLength,
				GNBValue:  targetID.RanNodeID.GNbID.GNBValue,
			},
		},
		Tai: &models.Tai{
			PlmnID: &models.PlmnID{
				Mcc: targetID.Tai.PlmnID.Mcc,
				Mnc: targetID.Tai.PlmnID.Mnc,
			},
			Tac: targetID.Tai.Tac,
		},
	}
	// amf changed in same plmn
	if amfid != "" {
		updateData.TargetServingNfID = amfid
	}
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverPrepared(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
	}
	updateData.HoState = models.HoStatePrepared
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverComplete(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, amfid string, guami *models.Guami) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	updateData.HoState = models.HoStateCompleted
	if amfid != "" {
		updateData.ServingNfID = amfid
		updateData.ServingNetwork = &models.PlmnID{
			Mcc: guami.PlmnID.Mcc,
			Mnc: guami.PlmnID.Mnc,
		}
		updateData.Guami = &models.Guami{
			PlmnID: &models.PlmnID{
				Mcc: guami.PlmnID.Mcc,
				Mnc: guami.PlmnID.Mnc,
			},
			AmfID: guami.AmfID,
		}
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2HandoverCanceled(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, cause context.CauseAll) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	updateData.HoState = models.HoStateCancelled
	if cause.Cause != nil {
		updateData.Cause = *cause.Cause
	}
	if cause.NgapCause != nil {
		updateData.NgApCause = &models.NgApCause{
			Group: cause.NgapCause.Group,
			Value: cause.NgapCause.Value,
		}
	}
	if cause.Var5GmmCause != nil {
		updateData.Var5gMmCauseValue = *cause.Var5GmmCause
	}
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextRequest(ctx ctxt.Context, smContext *context.SmContext, updateData models.SmContextUpdateData, n1Msg []byte, n2Info []byte) (*models.UpdateSmContextResponse, error) {
	var updateSmContextRequest models.UpdateSmContextRequest
	updateSmContextRequest.JSONData = &updateData
	updateSmContextRequest.BinaryDataN1SmMessage = n1Msg
	updateSmContextRequest.BinaryDataN2SmInformation = n2Info
	updateSmContextReponse, err := pdusession.UpdateSmContext(ctx, smContext.SmContextRef(), updateSmContextRequest)
	if err != nil {
		return updateSmContextReponse, fmt.Errorf("failed to update sm context: %s", err)
	}
	return updateSmContextReponse, nil
}
