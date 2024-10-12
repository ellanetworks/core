package producer

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mohae/deepcopy"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/ngap/ngapType"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/amf/consumer"
	"github.com/yeastengine/ella/internal/amf/context"
	gmm_message "github.com/yeastengine/ella/internal/amf/gmm/message"
	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/amf/nas"
	ngap_message "github.com/yeastengine/ella/internal/amf/ngap/message"
	"github.com/yeastengine/ella/internal/amf/util"
)

func SmContextHandler(s1, s2 string, msg interface{}) (interface{}, string, interface{}, interface{}) {
	switch msg := msg.(type) {
	case models.SmContextStatusNotification:
		var pduSessionID int
		if pduSessionIDTmp, err := strconv.Atoi(s2); err != nil {
			logger.ProducerLog.Warnf("PDU Session ID atoi failed: %+v", err)
		} else {
			pduSessionID = pduSessionIDTmp
		}
		r1 := SmContextStatusNotifyProcedure(s1, int32(pduSessionID), msg)
		return nil, "", r1, nil
	case models.PolicyUpdate:
		r1 := AmPolicyControlUpdateNotifyUpdateProcedure(s1, msg)
		return nil, "", r1, nil
	case models.TerminationNotification:
		r1 := AmPolicyControlUpdateNotifyTerminateProcedure(s1, msg)
		return nil, "", r1, nil
	}

	return nil, "", nil, nil
}

func HandleSmContextStatusNotify(request *httpwrapper.Request) *httpwrapper.Response {
	var ue *context.AmfUe
	var ok bool
	logger.ProducerLog.Infoln("[AMF] Handle SmContext Status Notify")

	guti := request.Params["guti"]
	pduSessionIDString := request.Params["pduSessionId"]

	amfSelf := context.AMF_Self()
	ue, ok = amfSelf.AmfUeFindByGuti(guti)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("Guti[%s] Not Found", guti),
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	smContextStatusNotification := request.Body.(models.SmContextStatusNotification)
	sbiMsg := context.SbiMsg{
		UeContextId: guti,
		ReqUri:      pduSessionIDString,
		Msg:         smContextStatusNotification,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	ue.EventChannel.UpdateSbiHandler(SmContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result
	// problemDetails := SmContextStatusNotifyProcedure(guti, int32(pduSessionID), smContextStatusNotification)
	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	}
}

func SmContextStatusNotifyProcedure(guti string, pduSessionID int32,
	smContextStatusNotification models.SmContextStatusNotification,
) *models.ProblemDetails {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByGuti(guti)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("Guti[%s] Not Found", guti),
		}
		return problemDetails
	}

	smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("PDUSessionID[%d] Not Found", pduSessionID),
		}
		return problemDetails
	}

	if smContextStatusNotification.StatusInfo.ResourceStatus == models.ResourceStatus_RELEASED {
		ue.ProducerLog.Debugf("Release PDU Session[%d] (Cause: %s)", pduSessionID,
			smContextStatusNotification.StatusInfo.Cause)

		if smContext.PduSessionIDDuplicated() {
			ue.ProducerLog.Debugf("Resume establishing PDU Session[%d]", pduSessionID)
			smContext.SetDuplicatedPduSessionID(false)
			go func() {
				var (
					snssai    models.Snssai
					dnn       string
					smMessage []byte
				)
				smMessage = smContext.ULNASTransport().GetPayloadContainerContents()

				if smContext.ULNASTransport().SNSSAI != nil {
					snssai = nasConvert.SnssaiToModels(smContext.ULNASTransport().SNSSAI)
				} else {
					if allowedNssai, ok := ue.AllowedNssai[smContext.AccessType()]; ok {
						snssai = *allowedNssai[0].AllowedSnssai
					} else {
						ue.GmmLog.Error("Ue doesn't have allowedNssai")
						return
					}
				}

				if smContext.ULNASTransport().DNN != nil {
					dnn = string(smContext.ULNASTransport().DNN.GetDNN())
				} else {
					if ue.SmfSelectionData != nil {
						snssaiStr := util.SnssaiModelsToHex(snssai)
						if snssaiInfo, ok := ue.SmfSelectionData.SubscribedSnssaiInfos[snssaiStr]; ok {
							for _, dnnInfo := range snssaiInfo.DnnInfos {
								if dnnInfo.DefaultDnnIndicator {
									dnn = dnnInfo.Dnn
								}
							}
						} else {
							// user's subscription context obtained from UDM does not contain the default DNN for the,
							// S-NSSAI, the AMF shall use a locally configured DNN as the DNN
							dnn = "internet"
						}
					}
				}

				newSmContext, cause, err := consumer.SelectSmf(ue, smContext.AccessType(), pduSessionID, snssai, dnn)
				if err != nil {
					logger.CallbackLog.Error(err)
					gmm_message.SendDLNASTransport(ue.RanUe[smContext.AccessType()],
						nasMessage.PayloadContainerTypeN1SMInfo,
						smContext.ULNASTransport().GetPayloadContainerContents(), pduSessionID, cause, nil, 0)
					return
				}

				response, smContextRef, errResponse, problemDetail, err := consumer.SendCreateSmContextRequest(
					ue, newSmContext, nil, smMessage)
				if response != nil {
					newSmContext.SetSmContextRef(smContextRef)
					newSmContext.SetUserLocation(deepcopy.Copy(ue.Location).(models.UserLocation))
					ue.GmmLog.Infof("create smContext[pduSessionID: %d] Success", pduSessionID)
					ue.StoreSmContext(pduSessionID, newSmContext)
					// TODO: handle response(response N2SmInfo to RAN if exists)
				} else if errResponse != nil {
					ue.ProducerLog.Warnf("PDU Session Establishment Request is rejected by SMF[pduSessionId:%d]\n", pduSessionID)
					gmm_message.SendDLNASTransport(ue.RanUe[smContext.AccessType()],
						nasMessage.PayloadContainerTypeN1SMInfo, errResponse.BinaryDataN1SmMessage, pduSessionID, 0, nil, 0)
				} else if err != nil {
					ue.ProducerLog.Errorf("Failed to Create smContext[pduSessionID: %d], Error[%s]\n", pduSessionID, err.Error())
				} else {
					ue.ProducerLog.Errorf("Failed to Create smContext[pduSessionID: %d], Error[%v]\n", pduSessionID, problemDetail)
				}
				smContext.DeleteULNASTransport()
			}()
		} else {
			ue.SmContextList.Delete(pduSessionID)
		}
	} else {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "INVALID_MSG_FORMAT",
			InvalidParams: []models.InvalidParam{
				{Param: "StatusInfo.ResourceStatus", Reason: "invalid value"},
			},
		}
		return problemDetails
	}
	return nil
}

func HandleAmPolicyControlUpdateNotifyUpdate(request *httpwrapper.Request) *httpwrapper.Response {
	var ue *context.AmfUe
	var ok bool
	logger.ProducerLog.Infoln("Handle AM Policy Control Update Notify [Policy update notification]")

	polAssoID := request.Params["polAssoId"]
	policyUpdate := request.Body.(models.PolicyUpdate)

	amfSelf := context.AMF_Self()
	ue, ok = amfSelf.AmfUeFindByPolicyAssociationID(polAssoID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("Policy Association ID[%s] Not Found", polAssoID),
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	sbiMsg := context.SbiMsg{
		UeContextId: polAssoID,
		ReqUri:      "",
		Msg:         policyUpdate,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	ue.EventChannel.UpdateSbiHandler(SmContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result
	// problemDetails := AmPolicyControlUpdateNotifyUpdateProcedure(polAssoID, policyUpdate)

	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	}
}

func AmPolicyControlUpdateNotifyUpdateProcedure(polAssoID string,
	policyUpdate models.PolicyUpdate,
) *models.ProblemDetails {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByPolicyAssociationID(polAssoID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("Policy Association ID[%s] Not Found", polAssoID),
		}
		return problemDetails
	}

	ue.AmPolicyAssociation.Triggers = policyUpdate.Triggers
	ue.RequestTriggerLocationChange = false

	for _, trigger := range policyUpdate.Triggers {
		if trigger == models.RequestTrigger_LOC_CH {
			ue.RequestTriggerLocationChange = true
		}
		//if trigger == models.RequestTrigger_PRA_CH {
		// TODO: Presence Reporting Area handling (TS 23.503 6.1.2.5, TS 23.501 5.6.11)
		//}
	}

	if policyUpdate.ServAreaRes != nil {
		ue.AmPolicyAssociation.ServAreaRes = policyUpdate.ServAreaRes
	}

	if policyUpdate.Rfsp != 0 {
		ue.AmPolicyAssociation.Rfsp = policyUpdate.Rfsp
	}

	if ue != nil {
		// use go routine to write response first to ensure the order of the procedure
		go func() {
			// UE is CM-Connected State
			if ue.CmConnect(models.AccessType__3_GPP_ACCESS) {
				gmm_message.SendConfigurationUpdateCommand(ue, models.AccessType__3_GPP_ACCESS, nil)
				// UE is CM-IDLE => paging
			} else {
				message, err := gmm_message.BuildConfigurationUpdateCommand(ue, models.AccessType__3_GPP_ACCESS, nil)
				if err != nil {
					logger.GmmLog.Errorf("Build Configuration Update Command Failed : %s", err.Error())
					return
				}

				ue.ConfigurationUpdateMessage = message
				ue.SetOnGoing(models.AccessType__3_GPP_ACCESS, &context.OnGoingProcedureWithPrio{
					Procedure: context.OnGoingProcedurePaging,
				})

				pkg, err := ngap_message.BuildPaging(ue, nil, false)
				if err != nil {
					logger.NgapLog.Errorf("Build Paging failed : %s", err.Error())
					return
				}
				ngap_message.SendPaging(ue, pkg)
			}
		}()
	}
	return nil
}

// TS 29.507 4.2.4.3
func HandleAmPolicyControlUpdateNotifyTerminate(request *httpwrapper.Request) *httpwrapper.Response {
	var ue *context.AmfUe
	logger.ProducerLog.Infoln("Handle AM Policy Control Update Notify [Request for termination of the policy association]")

	polAssoID := request.Params["polAssoId"]
	terminationNotification := request.Body.(models.TerminationNotification)

	amfSelf := context.AMF_Self()
	ue, ok := amfSelf.AmfUeFindByPolicyAssociationID(polAssoID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("Policy Association ID[%s] Not Found", polAssoID),
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	sbiMsg := context.SbiMsg{
		UeContextId: polAssoID,
		ReqUri:      "",
		Msg:         terminationNotification,
		Result:      make(chan context.SbiResponseMsg, 10),
	}
	ue.EventChannel.UpdateSbiHandler(SmContextHandler)
	ue.EventChannel.SubmitMessage(sbiMsg)
	msg := <-sbiMsg.Result

	// problemDetails := AmPolicyControlUpdateNotifyTerminateProcedure(polAssoID, terminationNotification)
	if msg.ProblemDetails != nil {
		return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
	} else {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	}
}

func AmPolicyControlUpdateNotifyTerminateProcedure(polAssoID string,
	terminationNotification models.TerminationNotification,
) *models.ProblemDetails {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByPolicyAssociationID(polAssoID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
			Detail: fmt.Sprintf("Policy Association ID[%s] Not Found", polAssoID),
		}
		return problemDetails
	}

	logger.CallbackLog.Infof("Cause of AM Policy termination[%+v]", terminationNotification.Cause)

	// use go routine to write response first to ensure the order of the procedure
	go func() {
		problem, err := consumer.AMPolicyControlDelete(ue)
		if problem != nil {
			logger.ProducerLog.Errorf("AM Policy Control Delete Failed Problem[%+v]", problem)
		} else if err != nil {
			logger.ProducerLog.Errorf("AM Policy Control Delete Error[%v]", err.Error())
		}
	}()
	return nil
}

// TS 23.502 4.2.2.2.3 Registration with AMF re-allocation
func HandleN1MessageNotify(request *httpwrapper.Request) *httpwrapper.Response {
	logger.ProducerLog.Infoln("[AMF] Handle N1 Message Notify")

	n1MessageNotify := request.Body.(models.N1MessageNotify)

	problemDetails := N1MessageNotifyProcedure(n1MessageNotify)
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	}
}

func N1MessageNotifyProcedure(n1MessageNotify models.N1MessageNotify) *models.ProblemDetails {
	logger.ProducerLog.Debugf("n1MessageNotify: %+v", n1MessageNotify)

	amfSelf := context.AMF_Self()

	registrationCtxtContainer := n1MessageNotify.JsonData.RegistrationCtxtContainer
	if registrationCtxtContainer.UeContext == nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "MANDATORY_IE_MISSING", // Defined in TS 29.500 5.2.7.2
			Detail: "Missing IE [UeContext] in RegistrationCtxtContainer",
		}
		return problemDetails
	}

	ran, ok := amfSelf.AmfRanFindByRanID(*registrationCtxtContainer.RanNodeId)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "MANDATORY_IE_INCORRECT",
			Detail: fmt.Sprintf("Can not find RAN[RanId: %+v]", *registrationCtxtContainer.RanNodeId),
		}
		return problemDetails
	}

	go func() {
		var amfUe *context.AmfUe
		ueContext := registrationCtxtContainer.UeContext
		if ueContext.Supi != "" {
			amfUe = amfSelf.NewAmfUe(ueContext.Supi)
		} else {
			amfUe = amfSelf.NewAmfUe("")
		}
		amfUe.CopyDataFromUeContextModel(*ueContext)

		ranUe := ran.RanUeFindByRanUeNgapID(int64(registrationCtxtContainer.AnN2ApId))

		ranUe.Location = *registrationCtxtContainer.UserLocation
		amfUe.Location = *registrationCtxtContainer.UserLocation
		ranUe.UeContextRequest = registrationCtxtContainer.UeContextRequest
		ranUe.OldAmfName = registrationCtxtContainer.InitialAmfName

		if registrationCtxtContainer.AllowedNssai != nil {
			allowedNssai := registrationCtxtContainer.AllowedNssai
			amfUe.AllowedNssai[allowedNssai.AccessType] = allowedNssai.AllowedSnssaiList
		}

		if len(registrationCtxtContainer.ConfiguredNssai) > 0 {
			amfUe.ConfiguredNssai = registrationCtxtContainer.ConfiguredNssai
		}

		amfUe.AttachRanUe(ranUe)

		nas.HandleNAS(ranUe, ngapType.ProcedureCodeInitialUEMessage, n1MessageNotify.BinaryDataN1Message)
	}()
	return nil
}

func HandleDeregistrationNotification(request *httpwrapper.Request) *httpwrapper.Response {
	logger.ProducerLog.Infoln("Handle Deregistration Notification")
	deregistrationData := request.Body.(models.DeregistrationData)

	switch deregistrationData.DeregReason {
	case "SUBSCRIPTION_WITHDRAWN":
		amfSelf := context.AMF_Self()
		if supi, exists := request.Params["supi"]; exists {
			reqUri := request.URL.RequestURI()
			if ue, ok := amfSelf.AmfUeFindBySupi(supi); ok {
				logger.ProducerLog.Debugln("amf ue found: ", ue.Supi)
				sbiMsg := context.SbiMsg{
					UeContextId: ue.Supi,
					ReqUri:      reqUri,
					Msg:         nil,
					Result:      make(chan context.SbiResponseMsg, 10),
				}
				ue.EventChannel.UpdateSbiHandler(HandleOAMPurgeUEContextRequest)
				ue.EventChannel.SubmitMessage(sbiMsg)
				msg := <-sbiMsg.Result
				if msg.ProblemDetails != nil {
					return httpwrapper.NewResponse(int(msg.ProblemDetails.(*models.ProblemDetails).Status), nil, msg.ProblemDetails.(*models.ProblemDetails))
				} else {
					return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
				}
			} else {
				return httpwrapper.NewResponse(http.StatusNotFound, nil, nil)
			}
		}

	case "":
		problemDetails := &models.ProblemDetails{
			Status: http.StatusBadRequest,
			Cause:  "MANDATORY_IE_MISSING", // Defined in TS 29.503 6.2.5.2
			Detail: "Missing IE [DeregReason] in DeregistrationData",
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)

	default:
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotImplemented,
			Cause:  "NOT_IMPLEMENTED", // Defined in TS 29.503
			Detail: "Unsupported [DeregReason] in DeregistrationData",
		}
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	return nil
}
