package consumer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Nsmf_PDUSession"
	"github.com/omec-project/openapi/models"
	amf_context "github.com/yeastengine/ella/internal/amf/context"
)

const N2SMINFO_ID = "N2SmInfo"

func SelectSmf(
	ue *amf_context.AmfUe,
	anType models.AccessType,
	pduSessionID int32,
	snssai models.Snssai,
	dnn string,
) (*amf_context.SmContext, uint8, error) {
	context := amf_context.AMF_Self()
	ue.GmmLog.Infof("Select SMF [snssai: %+v, dnn: %+v]", snssai, dnn)
	nsiInformation := ue.GetNsiInformationFromSnssai(anType, snssai)
	if nsiInformation == nil {
		ue.NssfUri = context.NssfUri
		response, problemDetails, err := NSSelectionGetForPduSession(ue, snssai)
		if err != nil {
			err = fmt.Errorf("NSSelection Get Error[%+v]", err)
			return nil, nasMessage.Cause5GMMPayloadWasNotForwarded, err
		} else if problemDetails != nil {
			err = fmt.Errorf("NSSelection Get Failed Problem[%+v]", problemDetails)
			return nil, nasMessage.Cause5GMMPayloadWasNotForwarded, err
		}
		nsiInformation = response.NsiInformation
	}

	smContext := amf_context.NewSmContext(pduSessionID)
	smContext.SetSnssai(snssai)
	smContext.SetDnn(dnn)
	smContext.SetAccessType(anType)

	if nsiInformation != nil {
		smContext.SetNsInstance(nsiInformation.NsiId)
	}

	smContext.SetSmfUri(context.SmfUri)
	return smContext, 0, nil
}

func SendCreateSmContextRequest(ue *amf_context.AmfUe, smContext *amf_context.SmContext,
	requestType *models.RequestType, nasPdu []byte) (
	response *models.PostSmContextsResponse, smContextRef string, errorResponse *models.PostSmContextsErrorResponse,
	problemDetail *models.ProblemDetails, err1 error,
) {
	smContextCreateData := buildCreateSmContextRequest(ue, smContext, nil)

	postSmContextsRequest := models.PostSmContextsRequest{
		JsonData:              &smContextCreateData,
		BinaryDataN1SmMessage: nasPdu,
	}

	configuration := Nsmf_PDUSession.NewConfiguration()
	configuration.SetBasePath(smContext.SmfUri())
	client := Nsmf_PDUSession.NewAPIClient(configuration)

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	postSmContextReponse, httpResponse, err := client.SMContextsCollectionApi.PostSmContexts(ctx, postSmContextsRequest)

	if err == nil {
		response = &postSmContextReponse
		smContextRef = httpResponse.Header.Get("Location")
	} else if httpResponse != nil {
		if httpResponse.Status != err.Error() {
			err1 = err
			return response, smContextRef, errorResponse, problemDetail, err1
		}
		switch httpResponse.StatusCode {
		case 400, 403, 404, 500, 503, 504:
			errResponse := err.(openapi.GenericOpenAPIError).Model().(models.PostSmContextsErrorResponse)
			errorResponse = &errResponse
		case 411, 413, 415, 429:
			problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
			problemDetail = &problem
		}
	} else {
		err1 = openapi.ReportError("server no response")
	}
	return response, smContextRef, errorResponse, problemDetail, err1
}

func buildCreateSmContextRequest(ue *amf_context.AmfUe, smContext *amf_context.SmContext,
	requestType *models.RequestType,
) (smContextCreateData models.SmContextCreateData) {
	context := amf_context.AMF_Self()
	smContextCreateData.Supi = ue.Supi
	smContextCreateData.UnauthenticatedSupi = ue.UnauthenticatedSupi
	smContextCreateData.Pei = ue.Pei
	smContextCreateData.Gpsi = ue.Gpsi
	smContextCreateData.PduSessionId = smContext.PduSessionID()
	snssai := smContext.Snssai()
	smContextCreateData.SNssai = &snssai
	smContextCreateData.Dnn = smContext.Dnn()
	smContextCreateData.ServingNfId = context.NfId
	guamiList := amf_context.GetServedGuamiList()
	smContextCreateData.Guami = &guamiList[0]
	// take seving networking plmn from userlocation.Tai
	if ue.Tai.PlmnId != nil {
		smContextCreateData.ServingNetwork = ue.Tai.PlmnId
	} else {
		ue.GmmLog.Warnf("Tai is not received from Serving Network, Serving Plmn [Mcc %v, Mnc: %v] is taken from Guami List", guamiList[0].PlmnId.Mcc, guamiList[0].PlmnId.Mnc)
		smContextCreateData.ServingNetwork = guamiList[0].PlmnId
	}
	if requestType != nil {
		smContextCreateData.RequestType = *requestType
	}
	smContextCreateData.N1SmMsg = new(models.RefToBinaryData)
	smContextCreateData.N1SmMsg.ContentId = "n1SmMsg"
	smContextCreateData.AnType = smContext.AccessType()
	if ue.RatType != "" {
		smContextCreateData.RatType = ue.RatType
	}
	// TODO: location is used in roaming scenerio
	// if ue.Location != nil {
	// 	smContextCreateData.UeLocation = ue.Location
	// }
	smContextCreateData.UeTimeZone = ue.TimeZone
	smContextCreateData.SmContextStatusUri = context.GetIPv4Uri() + "/namf-callback/v1/smContextStatus/" +
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
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, accessType models.AccessType) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxState_ACTIVATING
	if !amf_context.CompareUserLocation(ue.Location, smContext.UserLocation()) {
		updateData.UeLocation = &ue.Location
	}
	if smContext.AccessType() != accessType {
		updateData.AnType = smContext.AccessType()
	}
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if amf_context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceState_IN_AREA
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextDeactivateUpCnxState(ue *amf_context.AmfUe,
	smContext *amf_context.SmContext, cause amf_context.CauseAll) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.UpCnxState = models.UpCnxState_DEACTIVATED
	updateData.UeLocation = &ue.Location
	if cause.Cause != nil {
		updateData.Cause = *cause.Cause
	}
	if cause.NgapCause != nil {
		updateData.NgApCause = cause.NgapCause
	}
	if cause.Var5GmmCause != nil {
		updateData.Var5gMmCauseValue = *cause.Var5GmmCause
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextChangeAccessType(ue *amf_context.AmfUe,
	smContext *amf_context.SmContext, anTypeCanBeChanged bool) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.AnTypeCanBeChanged = anTypeCanBeChanged
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2Info(
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.N2SmInfoType = n2SmType
	updateData.N2SmInfo = new(models.RefToBinaryData)
	updateData.N2SmInfo.ContentId = N2SMINFO_ID
	updateData.UeLocation = &ue.Location
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandover(
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
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
		if amf_context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceState_IN_AREA
		} else {
			updateData.PresenceInLadn = models.PresenceState_OUT_OF_AREA
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextXnHandoverFailed(
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
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
	ue *amf_context.AmfUe,
	smContext *amf_context.SmContext,
	n2SmType models.N2SmInfoType,
	N2SmInfo []byte, amfid string, targetId *models.NgRanTargetId) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	if n2SmType != "" {
		updateData.N2SmInfoType = n2SmType
		updateData.N2SmInfo = new(models.RefToBinaryData)
		updateData.N2SmInfo.ContentId = N2SMINFO_ID
	}
	updateData.HoState = models.HoState_PREPARING
	updateData.TargetId = targetId
	// amf changed in same plmn
	if amfid != "" {
		updateData.TargetServingNfId = amfid
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, N2SmInfo)
}

func SendUpdateSmContextN2HandoverPrepared(
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, n2SmType models.N2SmInfoType, N2SmInfo []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
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
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, amfid string, guami *models.Guami) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.HoState = models.HoState_COMPLETED
	if amfid != "" {
		updateData.ServingNfId = amfid
		updateData.ServingNetwork = guami.PlmnId
		updateData.Guami = guami
	}
	if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
		if amf_context.InTaiList(ue.Tai, ladn.TaiLists) {
			updateData.PresenceInLadn = models.PresenceState_IN_AREA
		} else {
			updateData.PresenceInLadn = models.PresenceState_OUT_OF_AREA
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextN2HandoverCanceled(ue *amf_context.AmfUe,
	smContext *amf_context.SmContext, cause amf_context.CauseAll) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.HoState = models.HoState_CANCELLED
	if cause.Cause != nil {
		updateData.Cause = *cause.Cause
	}
	if cause.NgapCause != nil {
		updateData.NgApCause = cause.NgapCause
	}
	if cause.Var5GmmCause != nil {
		updateData.Var5gMmCauseValue = *cause.Var5GmmCause
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextHandoverBetweenAccessType(
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, targetAccessType models.AccessType, N1SmMsg []byte) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.AnType = targetAccessType
	if N1SmMsg != nil {
		updateData.N1SmMsg = new(models.RefToBinaryData)
		updateData.N1SmMsg.ContentId = "N1Msg"
	}
	return SendUpdateSmContextRequest(smContext, updateData, N1SmMsg, nil)
}

func SendUpdateSmContextHandoverBetweenAMF(
	ue *amf_context.AmfUe, smContext *amf_context.SmContext, amfid string, guami *models.Guami, activate bool) (
	*models.UpdateSmContextResponse, *models.UpdateSmContextErrorResponse, *models.ProblemDetails, error,
) {
	updateData := models.SmContextUpdateData{}
	updateData.ServingNfId = amfid
	updateData.ServingNetwork = guami.PlmnId
	updateData.Guami = guami
	if activate {
		updateData.UpCnxState = models.UpCnxState_ACTIVATING
		if !amf_context.CompareUserLocation(ue.Location, smContext.UserLocation()) {
			updateData.UeLocation = &ue.Location
		}
		if ladn, ok := ue.ServingAMF.LadnPool[smContext.Dnn()]; ok {
			if amf_context.InTaiList(ue.Tai, ladn.TaiLists) {
				updateData.PresenceInLadn = models.PresenceState_IN_AREA
			}
		}
	}
	return SendUpdateSmContextRequest(smContext, updateData, nil, nil)
}

func SendUpdateSmContextRequest(smContext *amf_context.SmContext,
	updateData models.SmContextUpdateData, n1Msg []byte, n2Info []byte) (
	response *models.UpdateSmContextResponse, errorResponse *models.UpdateSmContextErrorResponse,
	problemDetail *models.ProblemDetails, err1 error,
) {
	configuration := Nsmf_PDUSession.NewConfiguration()
	configuration.SetBasePath(smContext.SmfUri())
	client := Nsmf_PDUSession.NewAPIClient(configuration)

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	var updateSmContextRequest models.UpdateSmContextRequest
	updateSmContextRequest.JsonData = &updateData
	updateSmContextRequest.BinaryDataN1SmMessage = n1Msg
	updateSmContextRequest.BinaryDataN2SmInformation = n2Info

	updateSmContextReponse, httpResponse, err := client.IndividualSMContextApi.UpdateSmContext(ctx, smContext.SmContextRef(),
		updateSmContextRequest)

	if err == nil {
		response = &updateSmContextReponse
	} else if httpResponse != nil {
		if httpResponse.Status != err.Error() {
			err1 = err
			return response, errorResponse, problemDetail, err1
		}
		switch httpResponse.StatusCode {
		case 400, 403, 404, 500, 503:
			errResponse := err.(openapi.GenericOpenAPIError).Model().(models.UpdateSmContextErrorResponse)
			errorResponse = &errResponse
		case 411, 413, 415, 429:
			problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
			problemDetail = &problem
		}
	} else {
		err1 = openapi.ReportError("server no response")
	}
	return response, errorResponse, problemDetail, err1
}

// Release SmContext Request

func SendReleaseSmContextRequest(ue *amf_context.AmfUe, smContext *amf_context.SmContext,
	cause *amf_context.CauseAll, n2SmInfoType models.N2SmInfoType,
	n2Info []byte,
) (detail *models.ProblemDetails, err error) {
	configuration := Nsmf_PDUSession.NewConfiguration()
	configuration.SetBasePath(smContext.SmfUri())
	client := Nsmf_PDUSession.NewAPIClient(configuration)

	releaseData := buildReleaseSmContextRequest(ue, cause, n2SmInfoType, n2Info)
	releaseSmContextRequest := models.ReleaseSmContextRequest{
		JsonData: &releaseData,
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	response, err1 := client.IndividualSMContextApi.ReleaseSmContext(
		ctx, smContext.SmContextRef(), releaseSmContextRequest)

	if err1 == nil {
		ue.SmContextList.Delete(smContext.PduSessionID())
	} else if response != nil && response.Status == err1.Error() {
		problem := err1.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		detail = &problem
	} else {
		err = err1
	}
	return
}

func buildReleaseSmContextRequest(
	ue *amf_context.AmfUe, cause *amf_context.CauseAll, n2SmInfoType models.N2SmInfoType, n2Info []byte) (
	releaseData models.SmContextReleaseData,
) {
	if cause != nil {
		if cause.Cause != nil {
			releaseData.Cause = *cause.Cause
		}
		if cause.NgapCause != nil {
			releaseData.NgApCause = cause.NgapCause
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
	// TODO: other param(ueLocation...)
	return
}
