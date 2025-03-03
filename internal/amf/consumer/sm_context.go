// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
)

const N2SMINFO_ID = "N2SmInfo"

func SelectSmf(
	ue *context.AmfUe,
	anType models.AccessType,
	pduSessionID int32,
	snssai models.Snssai,
	dnn string,
) (*context.SmContext, uint8, error) {
	ue.GmmLog.Infof("Select SMF [snssai: %+v, dnn: %+v]", snssai, dnn)
	nsiInformation := ue.GetNsiInformationFromSnssai(anType, snssai)
	smContext := context.NewSmContext(pduSessionID)
	smContext.SetSnssai(snssai)
	smContext.SetDnn(dnn)
	smContext.SetAccessType(anType)

	if nsiInformation != nil {
		smContext.SetNsInstance(nsiInformation.NsiId)
	}

	return smContext, 0, nil
}

func SendCreateSmContextRequest(ue *context.AmfUe, smContext *context.SmContext, nasPdu []byte) (
	*models.PostSmContextsResponse, string, *models.PostSmContextsErrorResponse,
	*models.ProblemDetails, error,
) {
	smContextCreateData := buildCreateSmContextRequest(ue, smContext)
	postSmContextsRequest := models.PostSmContextsRequest{
		JsonData:              &smContextCreateData,
		BinaryDataN1SmMessage: nasPdu,
	}
	postSmContextReponse, smContextRef, postSmContextErrorReponse, err := pdusession.CreateSmContext(postSmContextsRequest)
	if err != nil {
		problemDetail := &models.ProblemDetails{
			Title:  "Create SmContext Request Error",
			Status: 500,
			Detail: err.Error(),
		}
		return nil, smContextRef, postSmContextErrorReponse, problemDetail, err
	}

	return postSmContextReponse, smContextRef, nil, nil, nil
}

func buildCreateSmContextRequest(ue *context.AmfUe, smContext *context.SmContext) (smContextCreateData models.SmContextCreateData) {
	amfSelf := context.AMF_Self()
	smContextCreateData.Supi = ue.Supi
	smContextCreateData.UnauthenticatedSupi = ue.UnauthenticatedSupi
	smContextCreateData.Pei = ue.Pei
	smContextCreateData.Gpsi = ue.Gpsi
	smContextCreateData.PduSessionId = smContext.PduSessionID()
	snssai := smContext.Snssai()
	smContextCreateData.SNssai = &models.Snssai{
		Sst: snssai.Sst,
		Sd:  snssai.Sd,
	}
	smContextCreateData.Dnn = smContext.Dnn()
	smContextCreateData.ServingNfId = amfSelf.NfId
	guamiList := context.GetServedGuamiList()
	smContextCreateData.Guami = &models.Guami{
		PlmnId: &models.PlmnId{
			Mcc: guamiList[0].PlmnId.Mcc,
			Mnc: guamiList[0].PlmnId.Mnc,
		},
		AmfId: guamiList[0].AmfId,
	}
	// take seving networking plmn from userlocation.Tai
	if ue.Tai.PlmnId != nil {
		smContextCreateData.ServingNetwork = &models.PlmnId{
			Mcc: ue.Tai.PlmnId.Mcc,
			Mnc: ue.Tai.PlmnId.Mnc,
		}
	} else {
		ue.GmmLog.Warnf("Tai is not received from Serving Network, Serving Plmn [Mcc %v, Mnc: %v] is taken from Guami List", guamiList[0].PlmnId.Mcc, guamiList[0].PlmnId.Mnc)
		smContextCreateData.ServingNetwork = &models.PlmnId{
			Mcc: guamiList[0].PlmnId.Mcc,
			Mnc: guamiList[0].PlmnId.Mnc,
		}
	}
	smContextCreateData.N1SmMsg = new(models.RefToBinaryData)
	smContextCreateData.N1SmMsg.ContentId = "n1SmMsg"
	smContextCreateData.AnType = smContext.AccessType()
	if ue.RatType != "" {
		smContextCreateData.RatType = ue.RatType
	}

	smContextCreateData.UeTimeZone = ue.TimeZone
	smContextCreateData.SmContextStatusUri = amfSelf.GetIPv4Uri() + "/namf-callback/v1/smContextStatus/" +
		ue.Guti + "/" + strconv.Itoa(int(smContext.PduSessionID()))

	return smContextCreateData
}

// Upadate SmContext Request
// servingNfId, smContextStatusUri, guami, servingNetwork -> amf change
// anType -> anType change
// ratType -> ratType change
// presenceInLadn -> Service Request , Xn handover, N2 handover and dnn is a ladn
// ueLocation -> the user location has changed or the user plane of the PDU session is deactivated
// upCnxState -> request the activation or the deactivation of the user plane connection of the PDU session
// hoState -> the preparation, execution or cancellation of a handover of the PDU session
// toBeSwitch -> Xn Handover to request to switch the PDU session to a new downlink N3 tunnel endpoint
// failedToBeSwitch -> indicate that the PDU session failed to be setup in the target RAN
// targetId, targetServingNfId(preparation with AMF change) -> N2 handover
// release -> duplicated PDU Session Id in subclause 5.2.2.3.11, slice not available in subclause 5.2.2.3.12
// ngApCause -> e.g. the NGAP cause for requesting to deactivate the user plane connection of the PDU session.
// 5gMmCauseValue -> AMF received a 5GMM cause code from the UE e.g 5GMM Status message in response to
// a Downlink NAS Transport message carrying 5GSM payload
// anTypeCanBeChanged

func SendUpdateSmContextActivateUpCnxState(
	ue *context.AmfUe, smContext *context.SmContext, accessType models.AccessType) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxState_ACTIVATING
	if !context.CompareUserLocation(ue.Location, smContext.UserLocation()) {
		updateData.UeLocation = &ue.Location
	}
	if smContext.AccessType() != accessType {
		updateData.AnType = smContext.AccessType()
	}
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceState_IN_AREA
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextDeactivateUpCnxState(ue *context.AmfUe,
	smContext *context.SmContext, cause context.CauseAll) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxState_DEACTIVATED
	updateData.UeLocation = &ue.Location
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
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextChangeAccessType(ue *context.AmfUe,
	smContext *context.SmContext, anTypeCanBeChanged bool) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.AnTypeCanBeChanged = anTypeCanBeChanged
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2Info(
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.N2SmInfoType = n2SmType
	updateData.N2SmInfo = new(models.RefToBinaryData)
	updateData.N2SmInfo.ContentId = N2SMINFO_ID
	updateData.UeLocation = &ue.Location
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandover(
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentId = N2SMINFO_ID
	}
	updateData.ToBeSwitched = true
	updateData.UeLocation = &ue.Location
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceState_IN_AREA
		} else {
			updateData.PresenceInLadn = models.PresenceState_OUT_OF_AREA
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandoverFailed(
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentId = N2SMINFO_ID
	}
	updateData.FailedToBeSwitched = true
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverPreparing(
	ue *context.AmfUe,
	smContext *context.SmContext,
	n2SmType models.N2SmInfoType,
	N2SmInfo []byte, amfid string, targetId *models.NgRanTargetId) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentId = N2SMINFO_ID
	}
	updateData.HoState = models.HoState_PREPARING
	updateData.TargetId = &models.NgRanTargetId{
		RanNodeId: &models.GlobalRanNodeId{
			PlmnId: &models.PlmnId{
				Mcc: targetId.RanNodeId.PlmnId.Mcc,
				Mnc: targetId.RanNodeId.PlmnId.Mnc,
			},
			GNbId: &models.GNbId{
				BitLength: targetId.RanNodeId.GNbId.BitLength,
				GNBValue:  targetId.RanNodeId.GNbId.GNBValue,
			},
		},
		Tai: &models.Tai{
			PlmnId: &models.PlmnId{
				Mcc: targetId.Tai.PlmnId.Mcc,
				Mnc: targetId.Tai.PlmnId.Mnc,
			},
			Tac: targetId.Tai.Tac,
		},
	}
	// amf changed in same plmn
	if amfid != "" {
		updateData.TargetServingNfId = amfid
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverPrepared(
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentId = N2SMINFO_ID
	}
	updateData.HoState = models.HoState_PREPARED
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverComplete(
	ue *context.AmfUe, smContext *context.SmContext, amfid string, guami *models.Guami) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.HoState = models.HoState_COMPLETED
	if amfid != "" {
		updateData.ServingNfId = amfid
		updateData.ServingNetwork = &models.PlmnId{
			Mcc: guami.PlmnId.Mcc,
			Mnc: guami.PlmnId.Mnc,
		}
		updateData.Guami = &models.Guami{
			PlmnId: &models.PlmnId{
				Mcc: guami.PlmnId.Mcc,
				Mnc: guami.PlmnId.Mnc,
			},
			AmfId: guami.AmfId,
		}
	}
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceState_IN_AREA
		} else {
			updateData.PresenceInLadn = models.PresenceState_OUT_OF_AREA
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2HandoverCanceled(ue *context.AmfUe,
	smContext *context.SmContext, cause context.CauseAll) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.HoState = models.HoState_CANCELLED
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
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextRequest(smContext *context.SmContext,
	updateData models.SmContextUpdateData, n1Msg []byte, n2Info []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse,
	error,
) {
	var updateSmContextRequest models.UpdateSmContextRequest
	updateSmContextRequest.JsonData = &updateData
	updateSmContextRequest.BinaryDataN1SmMessage = n1Msg
	updateSmContextRequest.BinaryDataN2SmInformation = n2Info

	updateSmContextReponse, err := pdusession.UpdateSmContext(smContext.SmContextRef(), updateSmContextRequest)
	if err != nil {
		return updateSmContextReponse, nil, fmt.Errorf("failed to update sm context: %s", err)
	}
	return updateSmContextReponse, nil, nil
}

func SendReleaseSmContextRequest(ue *context.AmfUe, smContext *context.SmContext,
	cause *context.CauseAll, n2SmInfoType models.N2SmInfoType,
	n2Info []byte,
) (detail *models.ProblemDetails, err error) {
	releaseData := buildReleaseSmContextRequest(ue, cause, n2SmInfoType, n2Info)
	releaseSmContextRequest := models.ReleaseSmContextRequest{
		JsonData: &releaseData,
	}
	err = pdusession.ReleaseSmContext(smContext.SmContextRef(), releaseSmContextRequest)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func buildReleaseSmContextRequest(
	ue *context.AmfUe, cause *context.CauseAll, n2SmInfoType models.N2SmInfoType, n2Info []byte) (
	releaseData models.SmContextReleaseData,
) {
	if cause != nil {
		if cause.Cause != nil {
			releaseData.Cause = *cause.Cause
		}
		if cause.NgapCause != nil {
			releaseData.NgApCause = &models.NgApCause{
				Group: cause.NgapCause.Group,
				Value: cause.NgapCause.Value,
			}
		}
		if cause.Var5GmmCause != nil {
			releaseData.Var5gMmCauseValue = *cause.Var5GmmCause
		}
	}
	if ue.TimeZone != "" {
		releaseData.UeTimeZone = ue.TimeZone
	}
	if n2Info != nil {
		releaseData.N2SmInfoType = n2SmInfoType
		releaseData.N2SmInfo = &models.RefToBinaryData{
			ContentId: N2SMINFO_ID,
		}
	}
	return
}
