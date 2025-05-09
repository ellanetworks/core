// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	ctx "context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"go.uber.org/zap"
)

const N2SMInfoID = "N2SmInfo"

func SelectSmf(
	ue *context.AmfUe,
	anType models.AccessType,
	pduSessionID int32,
	snssai models.Snssai,
	dnn string,
) *context.SmContext {
	nsiInformation := ue.GetNsiInformationFromSnssai(anType, snssai)
	smContext := context.NewSmContext(pduSessionID)
	smContext.SetSnssai(snssai)
	smContext.SetDnn(dnn)
	smContext.SetAccessType(anType)

	if nsiInformation != nil {
		smContext.SetNsInstance(nsiInformation.NsiID)
	}

	return smContext
}

func SendCreateSmContextRequest(ue *context.AmfUe, smContext *context.SmContext, nasPdu []byte, ctext ctx.Context) (string, *models.PostSmContextsErrorResponse, error) {
	smContextCreateData := buildCreateSmContextRequest(ue, smContext, ctext)
	postSmContextsRequest := models.PostSmContextsRequest{
		JSONData:              &smContextCreateData,
		BinaryDataN1SmMessage: nasPdu,
	}

	smContextRef, postSmContextErrorReponse, err := pdusession.CreateSmContext(postSmContextsRequest, ctext)
	if err != nil {
		return smContextRef, postSmContextErrorReponse, fmt.Errorf("create sm context request error: %s", err)
	}
	return smContextRef, nil, nil
}

func buildCreateSmContextRequest(ue *context.AmfUe, smContext *context.SmContext, ctext ctx.Context) (smContextCreateData models.SmContextCreateData) {
	amfSelf := context.AMFSelf()
	smContextCreateData.Supi = ue.Supi
	smContextCreateData.Pei = ue.Pei
	smContextCreateData.Gpsi = ue.Gpsi
	smContextCreateData.PduSessionID = smContext.PduSessionID()
	snssai := smContext.Snssai()
	smContextCreateData.SNssai = &models.Snssai{
		Sst: snssai.Sst,
		Sd:  snssai.Sd,
	}
	smContextCreateData.Dnn = smContext.Dnn()
	smContextCreateData.ServingNfID = amfSelf.NfID
	guamiList := context.GetServedGuamiList(ctext)
	smContextCreateData.Guami = &models.Guami{
		PlmnID: &models.PlmnID{
			Mcc: guamiList[0].PlmnID.Mcc,
			Mnc: guamiList[0].PlmnID.Mnc,
		},
		AmfID: guamiList[0].AmfID,
	}
	// take seving networking plmn from userlocation.Tai
	if ue.Tai.PlmnID != nil {
		smContextCreateData.ServingNetwork = &models.PlmnID{
			Mcc: ue.Tai.PlmnID.Mcc,
			Mnc: ue.Tai.PlmnID.Mnc,
		}
	} else {
		// ue.GmmLog.Warnf("Tai is not received from Serving Network, Serving Plmn [Mcc %v, Mnc: %v] is taken from Guami List", guamiList[0].PlmnID.Mcc, guamiList[0].PlmnID.Mnc)
		ue.GmmLog.Warn("Tai is not received from Serving Network, Serving Plmn is taken from Guami List", zap.String("mcc", guamiList[0].PlmnID.Mcc), zap.String("mnc", guamiList[0].PlmnID.Mnc))
		smContextCreateData.ServingNetwork = &models.PlmnID{
			Mcc: guamiList[0].PlmnID.Mcc,
			Mnc: guamiList[0].PlmnID.Mnc,
		}
	}
	smContextCreateData.N1SmMsg = new(models.RefToBinaryData)
	smContextCreateData.N1SmMsg.ContentID = "n1SmMsg"
	smContextCreateData.AnType = smContext.AccessType()
	if ue.RatType != "" {
		smContextCreateData.RatType = ue.RatType
	}

	smContextCreateData.UeTimeZone = ue.TimeZone
	smContextCreateData.SmContextStatusURI = amfSelf.GetIPv4Uri() + "/namf-callback/v1/smContextStatus/" +
		ue.Guti + "/" + strconv.Itoa(int(smContext.PduSessionID()))

	return smContextCreateData
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
	ue *context.AmfUe, smContext *context.SmContext, accessType models.AccessType, ctext ctx.Context) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxStateActivating
	if !context.CompareUserLocation(ue.Location, smContext.UserLocation()) {
		updateData.UeLocation = &ue.Location
	}
	if smContext.AccessType() != accessType {
		updateData.AnType = smContext.AccessType()
	}
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceStateInArea
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil, ctext)
}

func SendUpdateSmContextDeactivateUpCnxState(ue *context.AmfUe,
	smContext *context.SmContext, cause context.CauseAll, ctext ctx.Context) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxStateDeactivated
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
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil, ctext)
}

func SendUpdateSmContextChangeAccessType(ue *context.AmfUe,
	smContext *context.SmContext, anTypeCanBeChanged bool, ctext ctx.Context) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.AnTypeCanBeChanged = anTypeCanBeChanged
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil, ctext)
}

func SendUpdateSmContextN2Info(
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte, ctext ctx.Context) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.N2SmInfoType = n2SmType
	updateData.N2SmInfo = new(models.RefToBinaryData)
	updateData.N2SmInfo.ContentID = N2SMInfoID
	updateData.UeLocation = &ue.Location
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo, ctext)
}

func SendUpdateSmContextXnHandover(
	ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte, ctext ctx.Context) (
	*models.UpdateSmContextResponse, error,
) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentID = N2SMInfoID
	}
	updateData.ToBeSwitched = true
	updateData.UeLocation = &ue.Location
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceStateInArea
		} else {
			updateData.PresenceInLadn = models.PresenceStateOutOfArea
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo, ctext)
}

func SendUpdateSmContextXnHandoverFailed(ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte, ctext ctx.Context) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentID = N2SMInfoID
	}
	updateData.FailedToBeSwitched = true
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo, ctext)
}

func SendUpdateSmContextN2HandoverPreparing(ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte, amfid string, targetID *models.NgRanTargetID, ctext ctx.Context) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentID = N2SMInfoID
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
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo, ctext)
}

func SendUpdateSmContextN2HandoverPrepared(ue *context.AmfUe, smContext *context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte, ctext ctx.Context) (*models.UpdateSmContextResponse, error) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentID = N2SMInfoID
	}
	updateData.HoState = models.HoStatePrepared
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo, ctext)
}

func SendUpdateSmContextN2HandoverComplete(ue *context.AmfUe, smContext *context.SmContext, amfid string, guami *models.Guami, ctext ctx.Context) (*models.UpdateSmContextResponse, error) {
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
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceStateInArea
		} else {
			updateData.PresenceInLadn = models.PresenceStateOutOfArea
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil, ctext)
}

func SendUpdateSmContextN2HandoverCanceled(ue *context.AmfUe, smContext *context.SmContext, cause context.CauseAll, ctext ctx.Context) (*models.UpdateSmContextResponse, error) {
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
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil, ctext)
}

func SendUpdateSmContextRequest(smContext *context.SmContext, updateData models.SmContextUpdateData, n1Msg []byte, n2Info []byte, ctext ctx.Context) (*models.UpdateSmContextResponse, error) {
	var updateSmContextRequest models.UpdateSmContextRequest
	updateSmContextRequest.JSONData = &updateData
	updateSmContextRequest.BinaryDataN1SmMessage = n1Msg
	updateSmContextRequest.BinaryDataN2SmInformation = n2Info
	updateSmContextReponse, err := pdusession.UpdateSmContext(smContext.SmContextRef(), updateSmContextRequest, ctext)
	if err != nil {
		return updateSmContextReponse, fmt.Errorf("failed to update sm context: %s", err)
	}
	return updateSmContextReponse, nil
}
