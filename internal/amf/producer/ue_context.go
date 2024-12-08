package producer

import (
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/logger"
)

func UeContextHandler(s1, s2 string, msg interface{}) (interface{}, string, interface{}, interface{}) {
	switch msg := msg.(type) {
	case models.CreateUeContextRequest:
		r1, r2 := CreateUEContextProcedure(s1, msg)
		return r1, "", nil, r2
	case models.UeContextRelease:
		r1 := ReleaseUEContextProcedure(s1, msg)
		return nil, "", r1, nil
	case models.UeContextTransferRequest:
		r1, r2 := UEContextTransferProcedure(s1, msg)
		return r1, "", r2, nil
	case models.AssignEbiData:
		r1, r2, r3 := AssignEbiDataProcedure(s1, msg)
		return r1, "", r3, r2
	case models.UeRegStatusUpdateReqData:
		// r1, r2 := RegistrationStatusUpdateProcedure(s1, msg)
		// return r1, "", r2, nil
	}

	return nil, "", nil, nil
}

// TS 29.518 5.2.2.2.3
func HandleCreateUEContextRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Infof("Handle Create UE Context Request")

	createUeContextRequest := request.Body.(models.CreateUeContextRequest)
	ueContextID := request.Params["ueContextId"]

	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
	}
	sbiMsg := context.SbiMsg{
		UeContextId: ueContextID,
		ReqUri:      "",
		Msg:         createUeContextRequest,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	var createUeContextRspData *models.CreateUeContextResponse
	var ueContextCreateErr *models.UeContextCreateError
	ue.EventChannel.UpdateSbiHandler(UeContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result
	if msg.RespData != nil {
		createUeContextRspData = msg.RespData.(*models.CreateUeContextResponse)
	}
	if msg.TransferErr != nil {
		ueContextCreateErr = msg.TransferErr.(*models.UeContextCreateError)
	}
	// createUeContextResponse, ueContextCreateError := CreateUEContextProcedure(ueContextID, createUeContextRequest)
	if ueContextCreateErr != nil {
		return httpwrapper.NewResponse(int(ueContextCreateErr.Error.Status), nil, ueContextCreateErr)
	} else {
		return httpwrapper.NewResponse(http.StatusCreated, nil, createUeContextRspData)
	}
}

func CreateUEContextProcedure(ueContextID string, createUeContextRequest models.CreateUeContextRequest) (
	*models.CreateUeContextResponse, *models.UeContextCreateError,
) {
	amfSelf := context.AMF_Self()
	ueContextCreateData := createUeContextRequest.JsonData

	if ueContextCreateData.UeContext == nil || ueContextCreateData.TargetId == nil ||
		ueContextCreateData.PduSessionList == nil || ueContextCreateData.SourceToTargetData == nil ||
		ueContextCreateData.N2NotifyUri == "" {
		ueContextCreateError := &models.UeContextCreateError{
			Error: &models.ProblemDetails{
				Status: http.StatusForbidden,
				Cause:  "HANDOVER_FAILURE",
			},
		}
		return nil, ueContextCreateError
	}
	// create the UE context in target amf
	ue := amfSelf.NewAmfUe(ueContextID)
	// amfSelf.AmfRanSetByRanId(*ueContextCreateData.TargetId.RanNodeId)
	// ue.N1N2Message[ueContextId] = &context.N1N2Message{}
	// ue.N1N2Message[ueContextId].Request.JsonData = &models.N1N2MessageTransferReqData{
	// 	N2InfoContainer: &models.N2InfoContainer{
	// 		SmInfo: &models.N2SmInformation{
	// 			N2InfoContent: ueContextCreateData.SourceToTargetData,
	// 		},
	// 	},
	// }
	ue.HandoverNotifyUri = ueContextCreateData.N2NotifyUri

	amfSelf.AmfRanFindByRanID(*ueContextCreateData.TargetId.RanNodeId)
	supportedTAI := context.NewSupportedTAI()
	supportedTAI.Tai.Tac = ueContextCreateData.TargetId.Tai.Tac
	supportedTAI.Tai.PlmnId = ueContextCreateData.TargetId.Tai.PlmnId
	// ue.N1N2MessageSubscribeInfo[ueContextID] = &models.UeN1N2InfoSubscriptionCreateData{
	// 	N2NotifyCallbackUri: ueContextCreateData.N2NotifyUri,
	// }
	ue.UnauthenticatedSupi = ueContextCreateData.UeContext.SupiUnauthInd
	// should be smInfo list

	//for _, smInfo := range ueContextCreateData.PduSessionList {
	//if smInfo.N2InfoContent.NgapIeType == "NgapIeType_HANDOVER_REQUIRED" {
	// ue.N1N2Message[amfSelf.Uri].Request.JsonData.N2InfoContainer.SmInfo = &smInfo
	//}
	//}

	ue.RoutingIndicator = ueContextCreateData.UeContext.RoutingIndicator

	// optional
	ue.UdmGroupId = ueContextCreateData.UeContext.UdmGroupId
	ue.AusfGroupId = ueContextCreateData.UeContext.AusfGroupId
	// ueContextCreateData.UeContext.HpcfId
	ue.RatType = ueContextCreateData.UeContext.RestrictedRatList[0] // minItem = -1
	// ueContextCreateData.UeContext.ForbiddenAreaList
	// ueContextCreateData.UeContext.ServiceAreaRestriction
	// ueContextCreateData.UeContext.RestrictedCoreNwTypeList

	// it's not in 5.2.2.1.1 step 2a, so don't support
	// ue.Gpsi = ueContextCreateData.UeContext.GpsiList
	// ue.Pei = ueContextCreateData.UeContext.Pei
	// ueContextCreateData.UeContext.GroupList
	// ueContextCreateData.UeContext.DrxParameter
	// ueContextCreateData.UeContext.SubRfsp
	// ueContextCreateData.UeContext.UsedRfsp
	// ue.UEAMBR = ueContextCreateData.UeContext.SubUeAmbr
	// ueContextCreateData.UeContext.SmsSupport
	// ueContextCreateData.UeContext.SmsfId
	// ueContextCreateData.UeContext.SeafData
	// ueContextCreateData.UeContext.Var5gMmCapability
	// ueContextCreateData.UeContext.PcfId
	// ueContextCreateData.UeContext.PcfAmPolicyUri
	// ueContextCreateData.UeContext.AmPolicyReqTriggerList
	// ueContextCreateData.UeContext.EventSubscriptionList
	// ueContextCreateData.UeContext.MmContextList
	// ue.CurPduSession.PduSessionId = ueContextCreateData.UeContext.SessionContextList.
	// ue.TraceData = ueContextCreateData.UeContext.TraceData
	createUeContextResponse := new(models.CreateUeContextResponse)
	createUeContextResponse.JsonData = &models.UeContextCreatedData{
		UeContext: &models.UeContext{
			Supi: ueContextCreateData.UeContext.Supi,
		},
	}

	// response.JsonData.TargetToSourceData =
	// ue.N1N2Message[ueContextId].Request.JsonData.N2InfoContainer.SmInfo.N2InfoContent
	createUeContextResponse.JsonData.PduSessionList = ueContextCreateData.PduSessionList
	createUeContextResponse.JsonData.PcfReselectedInd = false
	// TODO: When  Target AMF selects a nw PCF for AM policy, set the flag to true.

	// response.UeContext = ueContextCreateData.UeContext
	// response.TargetToSourceData = ue.N1N2Message[amfSelf.Uri].Request.JsonData.N2InfoContainer.SmInfo.N2InfoContent
	// response.PduSessionList = ueContextCreateData.PduSessionList
	// response.PcfReselectedInd = false // TODO:When  Target AMF selects a nw PCF for AM policy, set the flag to true.

	// return httpwrapper.NewResponse(http.StatusCreated, nil, createUeContextResponse)
	return createUeContextResponse, nil
}

// TS 29.518 5.2.2.2.4
func HandleReleaseUEContextRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Info("Handle Release UE Context Request")

	ueContextRelease := request.Body.(models.UeContextRelease)
	ueContextID := request.Params["ueContextId"]
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
	}
	sbiMsg := context.SbiMsg{
		UeContextId: ueContextID,
		ReqUri:      "",
		Msg:         ueContextRelease,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	ue.EventChannel.UpdateSbiHandler(UeContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result

	// problemDetails := ReleaseUEContextProcedure(ueContextID, ueContextRelease)
	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	}
}

func ReleaseUEContextProcedure(ueContextID string, ueContextRelease models.UeContextRelease) *models.ProblemDetails {
	amfSelf := context.AMF_Self()

	// TODO: UE is emergency registered and the SUPI is not authenticated
	if ueContextRelease.Supi != "" {
		logger.GmmLog.Warnf("Emergency registered UE is not supported.")
		problemDetails := &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  "UNSPECIFIED",
		}
		return problemDetails
	}

	if ueContextRelease.NgapCause == nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "MANDATORY_IE_MISSING",
		}
		return problemDetails
	}

	logger.CommLog.Debugf("Release UE Context NGAP cause: %+v", ueContextRelease.NgapCause)

	if ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID); ok {
		ue.Remove()
	} else {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return problemDetails
	}

	return nil
}

// TS 29.518 5.2.2.2.1
func HandleUEContextTransferRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Info("Handle UE Context Transfer Request")

	ueContextTransferRequest := request.Body.(models.UeContextTransferRequest)
	ueContextID := request.Params["ueContextId"]

	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
	}
	sbiMsg := context.SbiMsg{
		UeContextId: ueContextID,
		ReqUri:      "",
		Msg:         ueContextTransferRequest,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	var ueContextTransferResponse *models.UeContextTransferResponse
	ue.EventChannel.UpdateSbiHandler(UeContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result
	if msg.RespData != nil {
		ueContextTransferResponse = msg.RespData.(*models.UeContextTransferResponse)
	}

	// ueContextTransferResponse, problemDetails := UEContextTransferProcedure(ueContextID, ueContextTransferRequest)
	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else {
		return httpwrapper.NewResponse(http.StatusOK, nil, ueContextTransferResponse)
	}
}

func UEContextTransferProcedure(ueContextID string, ueContextTransferRequest models.UeContextTransferRequest) (
	*models.UeContextTransferResponse, *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	if ueContextTransferRequest.JsonData == nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "MANDATORY_IE_MISSING",
		}
		return nil, problemDetails
	}

	UeContextTransferReqData := ueContextTransferRequest.JsonData

	if UeContextTransferReqData.AccessType == "" || UeContextTransferReqData.Reason == "" {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "MANDATORY_IE_MISSING",
		}
		return nil, problemDetails
	}

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return nil, problemDetails
	}

	ueContextTransferResponse := &models.UeContextTransferResponse{}
	ueContextTransferResponse.JsonData = new(models.UeContextTransferRspData)
	ueContextTransferRspData := ueContextTransferResponse.JsonData

	//if ue.GetAnType() != UeContextTransferReqData.AccessType {
	//for _, tai := range ue.RegistrationArea[ue.GetAnType()] {
	//if UeContextTransferReqData.PlmnId == tai.PlmnId {
	// TODO : generate N2 signalling
	//}
	//}
	//}

	switch UeContextTransferReqData.Reason {
	case models.TransferReason_INIT_REG:
		// TODO: check integrity of the registration request included in ueContextTransferRequest
		// TODO: handle condition of TS 29.518 5.2.2.2.1.1 step 2a case b
		ueContextTransferRspData.UeContext = buildUEContextModel(ue)
	case models.TransferReason_MOBI_REG:
		// TODO: check integrity of the registration request included in ueContextTransferRequest
		ueContextTransferRspData.UeContext = buildUEContextModel(ue)

		sessionContextList := &ueContextTransferRspData.UeContext.SessionContextList
		ue.SmContextList.Range(func(key, value interface{}) bool {
			smContext := value.(*context.SmContext)
			snssai := smContext.Snssai()
			pduSessionContext := models.PduSessionContext{
				PduSessionId: smContext.PduSessionID(),
				SmContextRef: smContext.SmContextRef(),
				SNssai:       &snssai,
				Dnn:          smContext.Dnn(),
				AccessType:   smContext.AccessType(),
				HsmfId:       smContext.HSmfID(),
				VsmfId:       smContext.VSmfID(),
				NsInstance:   smContext.NsInstance(),
			}
			*sessionContextList = append(*sessionContextList, pduSessionContext)
			return true
		})

		ueContextTransferRspData.UeRadioCapability = &models.N2InfoContent{
			NgapMessageType: 0,
			NgapIeType:      models.NgapIeType_UE_RADIO_CAPABILITY,
			NgapData: &models.RefToBinaryData{
				ContentId: "n2Info",
			},
		}
		b := []byte(ue.UeRadioCapability)
		copy(ueContextTransferResponse.BinaryDataN2Information, b)
	case models.TransferReason_MOBI_REG_UE_VALIDATED:
		ueContextTransferRspData.UeContext = buildUEContextModel(ue)

		sessionContextList := &ueContextTransferRspData.UeContext.SessionContextList
		ue.SmContextList.Range(func(key, value interface{}) bool {
			smContext := value.(*context.SmContext)
			snssai := smContext.Snssai()
			pduSessionContext := models.PduSessionContext{
				PduSessionId: smContext.PduSessionID(),
				SmContextRef: smContext.SmContextRef(),
				SNssai:       &snssai,
				Dnn:          smContext.Dnn(),
				AccessType:   smContext.AccessType(),
				HsmfId:       smContext.HSmfID(),
				VsmfId:       smContext.VSmfID(),
				NsInstance:   smContext.NsInstance(),
			}
			*sessionContextList = append(*sessionContextList, pduSessionContext)
			return true
		})

		ueContextTransferRspData.UeRadioCapability = &models.N2InfoContent{
			NgapMessageType: 0,
			NgapIeType:      models.NgapIeType_UE_RADIO_CAPABILITY,
			NgapData: &models.RefToBinaryData{
				ContentId: "n2Info",
			},
		}
		b := []byte(ue.UeRadioCapability)
		copy(ueContextTransferResponse.BinaryDataN2Information, b)
	default:
		logger.ProducerLog.Warnf("Invalid Transfer Reason: %+v", UeContextTransferReqData.Reason)
		problemDetails := &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  "MANDATORY_IE_INCORRECT",
			InvalidParams: []models.InvalidParam{
				{
					Param: "reason",
				},
			},
		}
		return nil, problemDetails
	}
	return ueContextTransferResponse, nil
}

func buildUEContextModel(ue *context.AmfUe) *models.UeContext {
	ueContext := new(models.UeContext)
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
			ueContext.SubUeAmbr = &models.Ambr{
				Uplink:   ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr.Uplink,
				Downlink: ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr.Downlink,
			}
		}
		if ue.AccessAndMobilitySubscriptionData.RfspIndex != 0 {
			ueContext.SubRfsp = ue.AccessAndMobilitySubscriptionData.RfspIndex
		}
	}

	if ue.AmPolicyUri != "" {
		ueContext.PcfAmPolicyUri = ue.AmPolicyUri
	}

	if ue.AmPolicyAssociation != nil {
		if len(ue.AmPolicyAssociation.Triggers) > 0 {
			ueContext.AmPolicyReqTriggerList = buildAmPolicyReqTriggers(ue.AmPolicyAssociation.Triggers)
		}
	}

	for _, eventSub := range ue.EventSubscriptionsInfo {
		if eventSub.EventSubscription != nil {
			ueContext.EventSubscriptionList = append(ueContext.EventSubscriptionList, *eventSub.EventSubscription)
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
		case models.RequestTrigger_LOC_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_LOCATION_CHANGE)
		case models.RequestTrigger_PRA_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_PRA_CHANGE)
		case models.RequestTrigger_SERV_AREA_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_SARI_CHANGE)
		case models.RequestTrigger_RFSP_CH:
			amPolicyReqTriggers = append(amPolicyReqTriggers, models.AmPolicyReqTrigger_RFSP_INDEX_CHANGE)
		}
	}
	return
}

// TS 29.518 5.2.2.6
func HandleAssignEbiDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Info("Handle Assign Ebi Data Request")

	assignEbiData := request.Body.(models.AssignEbiData)
	ueContextID := request.Params["ueContextId"]

	amfSelf := context.AMF_Self()

	// assignedEbiData, assignEbiError, problemDetails := AssignEbiDataProcedure(ueContextID, assignEbiData)
	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
	}
	sbiMsg := context.SbiMsg{
		UeContextId: ueContextID,
		ReqUri:      "",
		Msg:         assignEbiData,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	var assignEbiRspData *models.AssignedEbiData
	var assignEbiErr *models.AssignEbiError
	ue.EventChannel.UpdateSbiHandler(UeContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result
	if msg.RespData != nil {
		assignEbiRspData = msg.RespData.(*models.AssignedEbiData)
	}
	if msg.TransferErr != nil {
		assignEbiErr = msg.TransferErr.(*models.AssignEbiError)
	}

	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else if assignEbiErr != nil {
		return httpwrapper.NewResponse(int(assignEbiErr.Error.Status), nil, assignEbiErr)
	} else {
		return httpwrapper.NewResponse(http.StatusOK, nil, assignEbiRspData)
	}
}

func AssignEbiDataProcedure(ueContextID string, assignEbiData models.AssignEbiData) (
	*models.AssignedEbiData, *models.AssignEbiError, *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return nil, nil, problemDetails
	}

	// TODO: AssignEbiError not used, check it!
	if _, ok := ue.SmContextFindByPDUSessionID(assignEbiData.PduSessionId); ok {
		assignedEbiData := &models.AssignedEbiData{}
		assignedEbiData.PduSessionId = assignEbiData.PduSessionId
		return assignedEbiData, nil, nil
	} else {
		logger.ProducerLog.Errorln("ue.SmContextList is nil")
		return nil, nil, nil
	}
}

// TS 29.518 5.2.2.2.2
func HandleRegistrationStatusUpdateRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Info("Handle Registration Status Update Request")

	ueRegStatusUpdateReqData := request.Body.(models.UeRegStatusUpdateReqData)
	ueContextID := request.Params["ueContextId"]

	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
	}
	sbiMsg := context.SbiMsg{
		UeContextId: ueContextID,
		ReqUri:      "",
		Msg:         ueRegStatusUpdateReqData,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	var ueRegStatusUpdateRspData *models.UeRegStatusUpdateRspData
	ue.EventChannel.UpdateSbiHandler(UeContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result
	if msg.RespData != nil {
		ueRegStatusUpdateRspData = msg.RespData.(*models.UeRegStatusUpdateRspData)
	}
	// ueRegStatusUpdateRspData, problemDetails := RegistrationStatusUpdateProcedure(ueContextID, ueRegStatusUpdateReqData)
	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else {
		return httpwrapper.NewResponse(http.StatusOK, nil, ueRegStatusUpdateRspData)
	}
}
