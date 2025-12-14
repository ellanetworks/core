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
	ue *context.AmfUe, smContext *context.SmContext) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := &models.SmContextUpdateData{
		UpCnxState: models.UpCnxStateActivating,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextDeactivateUpCnxState(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext) (*models.UpdateSmContextResponse, error) {
	updateData := &models.SmContextUpdateData{
		UpCnxState: models.UpCnxStateDeactivated,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2Info(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (*models.UpdateSmContextResponse, error) {
	updateData := &models.SmContextUpdateData{
		N2SmInfoType: n2SmType,
	}
	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandover(
	ctx ctxt.Context,
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := &models.SmContextUpdateData{
		N2SmInfoType: n2SmType,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandoverFailed(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (*models.UpdateSmContextResponse, error) {
	updateData := &models.SmContextUpdateData{
		N2SmInfoType: n2SmType,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverPreparing(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (*models.UpdateSmContextResponse, error) {
	updateData := &models.SmContextUpdateData{
		N2SmInfoType: n2SmType,
		HoState:      models.HoStatePreparing,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverPrepared(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (*models.UpdateSmContextResponse, error) {
	updateData := &models.SmContextUpdateData{
		N2SmInfoType: n2SmType,
		HoState:      models.HoStatePrepared,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverComplete(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext) (*models.UpdateSmContextResponse, error) {
	updateData := &models.SmContextUpdateData{
		HoState: models.HoStateCompleted,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2HandoverCanceled(ctx ctxt.Context, ue *context.AmfUe, smContext *context.SmContext) (*models.UpdateSmContextResponse, error) {
	updateData := &models.SmContextUpdateData{
		HoState: models.HoStateCancelled,
	}

	return SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
}

func SendUpdateSmContextRequest(ctx ctxt.Context, smContext *context.SmContext, updateData *models.SmContextUpdateData, n1Msg []byte, n2Info []byte) (*models.UpdateSmContextResponse, error) {
	updateSmContextRequest := models.UpdateSmContextRequest{
		JSONData:                  updateData,
		BinaryDataN1SmMessage:     n1Msg,
		BinaryDataN2SmInformation: n2Info,
	}

	updateSmContextReponse, err := pdusession.UpdateSmContext(ctx, smContext.SmContextRef(), updateSmContextRequest)
	if err != nil {
		return updateSmContextReponse, fmt.Errorf("failed to update sm context: %s", err)
	}

	return updateSmContextReponse, nil
}
