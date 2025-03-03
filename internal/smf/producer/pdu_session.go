// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"fmt"
	"net/http"

	amf_producer "github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/consumer"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/ellanetworks/core/internal/smf/util"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/openapi"
)

func formContextCreateErrRsp(httpStatus int, problemBody *models.ProblemDetails, n1SmMsg *models.RefToBinaryData) *util.Response {
	return &util.Response{
		Header: nil,
		Status: httpStatus,
		Body: models.PostSmContextsErrorResponse{
			JsonData: &models.SmContextCreateError{
				Error:   problemBody,
				N1SmMsg: n1SmMsg,
			},
		},
	}
}

func HandlePduSessionContextReplacement(smCtxtRef string) error {
	smCtxt := context.GetSMContext(smCtxtRef)

	if smCtxt != nil {
		smCtxt.SMLock.Lock()

		smCtxt.LocalPurged = true

		context.RemoveSMContext(smCtxt.Ref)

		// Check if UPF session set, send release
		if smCtxt.Tunnel != nil {
			releaseTunnel(smCtxt)
		}

		smCtxt.SMLock.Unlock()
	}

	return nil
}

func HandlePDUSessionSMContextCreate(request models.PostSmContextsRequest, smContext *context.SMContext) (*util.Response, error) {
	// GSM State
	// PDU Session Establishment Accept/Reject
	var response models.PostSmContextsResponse
	response.JsonData = new(models.SmContextCreatedData)

	// Check has PDU Session Establishment Request
	m := nas.NewMessage()
	if err := m.GsmMessageDecode(&request.BinaryDataN1SmMessage); err != nil ||
		m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		logger.SmfLog.Errorln("PDUSessionSMContextCreate, GsmMessageDecode Error: ", err)
		response := formContextCreateErrRsp(http.StatusForbidden, &models.N1SmError, nil)
		return response, fmt.Errorf("GsmMsgDecodeError")
	}

	createData := request.JsonData

	// Create SM context
	// smContext := context.NewSMContext(createData.Supi, createData.PduSessionId)
	smContext.SubPduSessLog.Infof("SM context created")
	// smContext.ChangeState(context.SmStateActivePending)
	smContext.SetCreateData(createData)
	smContext.SmStatusNotifyUri = createData.SmContextStatusUri

	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	// DNN Information from config
	smContext.DNNInfo = context.RetrieveDnnInformation(*createData.SNssai, createData.Dnn)
	if smContext.DNNInfo == nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, S-NSSAI[sst: %d, sd: %s] DNN[%s] does not match DNN Config",
			createData.SNssai.Sst, createData.SNssai.Sd, createData.Dnn)
		problemDetails := &models.ProblemDetails{
			Title:         "DNN Denied",
			Status:        http.StatusForbidden,
			Detail:        "The subscriber does not have the necessary subscription to access the DNN",
			Cause:         "DNN_DENIED",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusForbidden, problemDetails, nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		return response, fmt.Errorf("SnssaiError")
	}

	// IP Allocation
	smfSelf := context.SMF_Self()
	if ip, err := smfSelf.DbInstance.AllocateIP(smContext.Supi); err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, failed allocate IP address: ", err)
		problemDetails := &models.ProblemDetails{
			Title:         "IP Allocation Error",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to insufficient resources for the IP allocation.",
			Cause:         "INSUFFICIENT_RESOURCES",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMInsufficientResources)
		return response, fmt.Errorf("failed allocate IP address: %v", err)
	} else {
		smContext.PDUAddress = &context.UeIpAddr{Ip: ip, UpfProvided: false}
		smContext.SubPduSessLog.Infof("Successful IP Allocation: %s",
			smContext.PDUAddress.Ip.String())
	}

	snssai := openapi.MarshToJsonString(createData.SNssai)[0]

	sessSubData, err := udm.GetAndSetSmData(smContext.Supi, createData.Dnn, snssai)
	if err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, get SessionManagementSubscriptionData error: ", err)
		problemDetails := &models.ProblemDetails{
			Title:         "Subscription Data Fetch error",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to failure in fetching subscription data.",
			Cause:         "REQUEST_REJECTED",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMRequestRejectedUnspecified)
		return response, fmt.Errorf("SubscriptionError")
	}
	if len(sessSubData) > 0 {
		smContext.DnnConfiguration = sessSubData[0].DnnConfigurations[createData.Dnn]
		smContext.SubPduSessLog.Infof("subscription data retrieved from UDM")
	} else {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SessionManagementSubscriptionData from UDM is nil")
		problemDetails := &models.ProblemDetails{
			Title:         "Subscription Data Fetch error",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to not receiving any subscription data.  ",
			Cause:         "REQUEST_REJECTED",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMRequestRejectedUnspecified)
		return response, fmt.Errorf("NoSubscriptionError")
	}

	// Decode UE content(PCO)
	establishmentRequest := m.PDUSessionEstablishmentRequest
	smContext.HandlePDUSessionEstablishmentRequest(establishmentRequest)

	smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, send NF Discovery Serving PCF success")

	// PCF Policy Association
	var smPolicyDecision *models.SmPolicyDecision
	if smPolicyDecisionRsp, httpStatus, err := consumer.SendSMPolicyAssociationCreate(smContext); err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SMPolicyAssociationCreate error: ", err)
		problemDetails := &models.ProblemDetails{
			Title:         "PCF Discovery Failure",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to failure in creating PCF policy.",
			Cause:         "REQUEST_REJECTED",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMRequestRejectedUnspecified)
		return response, fmt.Errorf("PcfAssoError")
	} else if httpStatus != http.StatusCreated {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SMPolicyAssociationCreate http status: ", http.StatusText(httpStatus))
		problemDetails := &models.ProblemDetails{
			Title:         "PCF Discovery Failure",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to failure in creating PCF policy.",
			Cause:         "REQUEST_REJECTED",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMRequestRejectedUnspecified)
		return response, fmt.Errorf("PcfAssoError")
	} else {
		smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, Policy association create success")
		smPolicyDecision = smPolicyDecisionRsp

		policyUpdates := qos.BuildSmPolicyUpdate(&smContext.SmPolicyData, smPolicyDecision)
		smContext.SmPolicyUpdates = append(smContext.SmPolicyUpdates, policyUpdates)
	}

	// dataPath selection
	smContext.Tunnel = context.NewUPTunnel()
	var defaultPath *context.DataPath
	upfSelectionParams := &context.UPFSelectionParams{
		Dnn: createData.Dnn,
		SNssai: &context.SNssai{
			Sst: createData.SNssai.Sst,
			Sd:  createData.SNssai.Sd,
		},
	}

	defaultUPPath, err := context.GetUserPlaneInformation().GetDefaultUserPlanePathByDNN(upfSelectionParams)
	if err != nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, get default UP path error: %v", err.Error())
		problemDetails := &models.ProblemDetails{
			Title:         "UPF Data Path Failure",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to failure in fetching UPF data path.",
			Cause:         "REQUEST_REJECTED",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMRequestRejectedUnspecified)
		return response, fmt.Errorf("DataPathError")
	}
	defaultPath, err = context.GenerateDataPath(defaultUPPath, smContext)
	if err != nil {
		smContext.SubPduSessLog.Errorf("couldn't generate data path: %v", err.Error())
		problemDetails := &models.ProblemDetails{
			Title:         "UPF Data Path Failure",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to failure in fetching UPF data path.",
			Cause:         "REQUEST_REJECTED",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMRequestRejectedUnspecified)
		return response, fmt.Errorf("DataPathError")
	}
	if defaultPath != nil {
		defaultPath.IsDefaultPath = true
		smContext.Tunnel.AddDataPath(defaultPath)

		if err := defaultPath.ActivateTunnelAndPDR(smContext, 255); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, data path error: %v", err.Error())
			problemDetails := &models.ProblemDetails{
				Title:         "UPF Data Path Failure",
				Status:        http.StatusInternalServerError,
				Detail:        "The request cannot be provided due to failure in fetching UPF data path.",
				Cause:         "REQUEST_REJECTED",
				InvalidParams: nil,
			}
			response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMRequestRejectedUnspecified)
			return response, fmt.Errorf("DataPathError")
		}
	}
	if defaultPath == nil {
		smContext.ChangeState(context.SmStateInit)
		problemDetails := &models.ProblemDetails{
			Title:         "DNN Resource insufficient",
			Status:        http.StatusInternalServerError,
			Detail:        "The request cannot be provided due to insufficient resources for the specific slice and DNN.",
			Cause:         "INSUFFICIENT_RESOURCES_SLICE_DNN",
			InvalidParams: nil,
		}
		response := smContext.GeneratePDUSessionEstablishmentReject(http.StatusInternalServerError, problemDetails, nasMessage.Cause5GSMInsufficientResourcesForSpecificSliceAndDNN)
		return response, fmt.Errorf("default data path not found")
	}

	response.JsonData = smContext.BuildCreatedData()
	httpResponse := &util.Response{
		Header: http.Header{
			"Location": {smContext.Ref},
		},
		Status: http.StatusCreated,
		Body:   response,
	}

	smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, PDU session context create success ")

	return httpResponse, nil
}

func HandlePDUSessionSMContextUpdate(request models.UpdateSmContextRequest, smContext *context.SMContext) (*util.Response, error) {
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	pfcpAction := &pfcpAction{}
	var response models.UpdateSmContextResponse
	response.JsonData = new(models.SmContextUpdatedData)

	var httpResponse *util.Response
	httpResponse, err := HandleUpdateN1Msg(request, smContext, &response, pfcpAction)
	if err != nil {
		return httpResponse, err
	}

	pfcpParam := &pfcpParam{
		pdrList: []*context.PDR{},
		farList: []*context.FAR{},
		barList: []*context.BAR{},
		qerList: []*context.QER{},
	}

	// UP Cnx State handling
	if err := HandleUpCnxState(request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return httpResponse, err
	}

	// N2 Msg Handling
	if err := HandleUpdateN2Msg(request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return httpResponse, err
	}

	// Ho state handling
	if err := HandleUpdateHoState(request, smContext, &response); err != nil {
		return httpResponse, err
	}

	// Cause handling
	if err := HandleUpdateCause(request, smContext, &response, pfcpAction); err != nil {
		return httpResponse, err
	}

	switch smContext.SMContextState {
	case context.SmStatePfcpModify:

		var err error

		// Initiate PFCP Delete
		if pfcpAction.sendPfcpDelete {
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send PFCP Deletion")
			smContext.ChangeState(context.SmStatePfcpRelease)

			// Initiate PFCP Release
			if err = SendPfcpSessionReleaseReq(smContext); err != nil {
				smContext.SubCtxLog.Errorf("pfcp session release error: %v ", err.Error())
			}

			// Change state to InactivePending
			smContext.ChangeState(context.SmStateInActivePending)

			// Update response to success
			httpResponse = &util.Response{
				Status: http.StatusOK,
				Body:   response,
			}
		} else if pfcpAction.sendPfcpModify {
			smContext.ChangeState(context.SmStatePfcpModify)
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send PFCP Modification")

			// Initiate PFCP Modify
			if err = SendPfcpSessionModifyReq(smContext, pfcpParam); err != nil {
				// Modify failure
				smContext.SubCtxLog.Errorf("pfcp session modify error: %v ", err.Error())

				// Form Modify err rsp
				httpResponse = makePduCtxtModifyErrRsp(smContext, err.Error())
			} else {
				// Modify Success
				httpResponse = &util.Response{
					Status: http.StatusOK,
					Body:   response,
				}

				smContext.ChangeState(context.SmStateActive)
			}
		}

	case context.SmStateModify:
		smContext.ChangeState(context.SmStateActive)
		httpResponse = &util.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	case context.SmStateInit, context.SmStateInActivePending:
		httpResponse = &util.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	default:
		smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SM Context State [%s] shouldn't be here\n", smContext.SMContextState)
		httpResponse = &util.Response{
			Status: http.StatusOK,
			Body:   response,
		}
	}

	return httpResponse, nil
}

func makePduCtxtModifyErrRsp(smContext *context.SMContext, errStr string) *util.Response {
	problemDetail := models.ProblemDetails{
		Title:  errStr,
		Status: http.StatusInternalServerError,
		Detail: errStr,
		Cause:  "UPF_NOT_RESPONDING",
	}
	var n1buf, n2buf []byte
	var err error
	if n1buf, err = context.BuildGSMPDUSessionReleaseCommand(smContext); err != nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build GSM PDUSessionReleaseCommand failed: %+v", err)
	}

	if n2buf, err = context.BuildPDUSessionResourceReleaseCommandTransfer(smContext); err != nil {
		smContext.SubPduSessLog.Errorf("PDUSessionSMContextUpdate, build PDUSessionResourceReleaseCommandTransfer failed: %+v", err)
	}

	// It is just a template
	httpResponse := &util.Response{
		Status: http.StatusServiceUnavailable,
		Body: models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error:        &problemDetail,
				N1SmMsg:      &models.RefToBinaryData{ContentId: context.PDU_SESS_REL_CMD},
				N2SmInfo:     &models.RefToBinaryData{ContentId: context.PDU_SESS_REL_CMD},
				N2SmInfoType: models.N2SmInfoType_PDU_RES_REL_CMD,
			},
			BinaryDataN1SmMessage:     n1buf,
			BinaryDataN2SmInformation: n2buf,
		}, // Depends on the reason why N4 fail
	}

	return httpResponse
}

func HandlePDUSessionSMContextRelease(body models.ReleaseSmContextRequest, smContext *context.SMContext) (*util.Response, error) {
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	// Send Policy delete
	if httpStatus, err := consumer.SendSMPolicyAssociationDelete(smContext, &body); err != nil {
		smContext.SubCtxLog.Errorf("SM policy delete error [%v] ", err.Error())
	} else {
		smContext.SubCtxLog.Infof("SM policy delete success with http status [%v] ", httpStatus)
	}

	// Release UE IP-Address
	err := smContext.ReleaseUeIpAddr()
	if err != nil {
		smContext.SubPduSessLog.Errorf("release UE IP address failed: %v", err)
	}

	// Initiate PFCP release
	smContext.ChangeState(context.SmStatePfcpRelease)

	var httpResponse *util.Response

	// Release User-plane
	status, ok := releaseTunnel(smContext)
	if !ok {
		// already released
		httpResponse = &util.Response{
			Status: http.StatusNoContent,
			Body:   nil,
		}
		context.RemoveSMContext(smContext.Ref)
		logger.SmfLog.Warnf("Removed SM Context due to release: %s", smContext.Ref)
		return httpResponse, nil
	}

	switch *status {
	case context.SessionReleaseSuccess:
		smContext.ChangeState(context.SmStatePfcpRelease)
		httpResponse = &util.Response{
			Status: http.StatusNoContent,
			Body:   nil,
		}

	case context.SessionReleaseTimeout:
		smContext.ChangeState(context.SmStateActive)
		httpResponse = &util.Response{
			Status: int(http.StatusInternalServerError),
		}

	case context.SessionReleaseFailed:
		// Update SmContext Request(N1 PDU Session Release Request)
		// Send PDU Session Release Reject
		problemDetail := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILULE",
		}
		httpResponse = &util.Response{
			Status: int(problemDetail.Status),
		}
		smContext.ChangeState(context.SmStateActive)
		errResponse := models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error: &problemDetail,
			},
		}
		if buf, err := context.BuildGSMPDUSessionReleaseReject(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextRelease, build GSM PDUSessionReleaseReject failed: %+v", err)
		} else {
			errResponse.BinaryDataN1SmMessage = buf
		}

		errResponse.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseReject"}
		httpResponse.Body = errResponse
	default:
		smContext.SubCtxLog.Warnf("PDUSessionSMContextRelease, The state shouldn't be [%s]\n", status)

		problemDetail := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILULE",
		}
		httpResponse = &util.Response{
			Status: int(problemDetail.Status),
		}
		smContext.ChangeState(context.SmStateActive)
		errResponse := models.UpdateSmContextErrorResponse{
			JsonData: &models.SmContextUpdateError{
				Error: &problemDetail,
			},
		}
		if buf, err := context.BuildGSMPDUSessionReleaseReject(smContext); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextRelease, build GSM PDUSessionReleaseReject failed: %+v", err)
		} else {
			errResponse.BinaryDataN1SmMessage = buf
		}

		errResponse.JsonData.N1SmMsg = &models.RefToBinaryData{ContentId: "PDUSessionReleaseReject"}
		httpResponse.Body = errResponse
	}

	context.RemoveSMContext(smContext.Ref)
	logger.SmfLog.Warnf("Removed SM Context due to release: %s", smContext.Ref)

	return httpResponse, nil
}

func releaseTunnel(smContext *context.SMContext) (*context.PFCPSessionResponseStatus, bool) {
	if smContext.Tunnel == nil {
		smContext.SubPduSessLog.Errorf("releaseTunnel, pfcp tunnel already released")
		return nil, false
	}
	var responseStatus *context.PFCPSessionResponseStatus
	deletedPFCPNode := make(map[string]bool)
	smContext.PendingUPF = make(context.PendingUPF)
	for _, dataPath := range smContext.Tunnel.DataPathPool {
		dataPath.DeactivateTunnelAndPDR(smContext)
		for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
			curUPFID := curDataPathNode.UPF.UUID()
			if _, exist := deletedPFCPNode[curUPFID]; !exist {
				status, err := pfcp.SendPfcpSessionDeletionRequest(curDataPathNode.UPF.NodeID, smContext)
				responseStatus = status
				if err != nil {
					smContext.SubPduSessLog.Errorf("releaseTunnel, send PFCP session deletion request failed: %v", err)
				}
				deletedPFCPNode[curUPFID] = true
				smContext.PendingUPF[curDataPathNode.GetNodeIP()] = true
			}
		}
	}
	smContext.Tunnel = nil
	return responseStatus, true
}

func SendPduSessN1N2Transfer(smContext *context.SMContext, success bool) error {
	// N1N2 Request towards AMF
	n1n2Request := models.N1N2MessageTransferRequest{}

	// N2 Container Info
	n2InfoContainer := models.N2InfoContainer{
		N2InformationClass: models.N2InformationClass_SM,
		SmInfo: &models.N2SmInformation{
			PduSessionId: smContext.PDUSessionID,
			N2InfoContent: &models.N2InfoContent{
				NgapIeType: models.NgapIeType_PDU_RES_SETUP_REQ,
				NgapData: &models.RefToBinaryData{
					ContentId: "N2SmInformation",
				},
			},
			SNssai: smContext.Snssai,
		},
	}

	// N1 Container Info
	n1MsgContainer := models.N1MessageContainer{
		N1MessageClass:   "SM",
		N1MessageContent: &models.RefToBinaryData{ContentId: "GSM_NAS"},
	}

	// N1N2 Json Data
	n1n2Request.JsonData = &models.N1N2MessageTransferReqData{PduSessionId: smContext.PDUSessionID}

	if success {
		if smNasBuf, err := context.BuildGSMPDUSessionEstablishmentAccept(smContext); err != nil {
			logger.SmfLog.Errorf("Build GSM PDUSessionEstablishmentAccept failed: %s", err)
		} else {
			n1n2Request.BinaryDataN1Message = smNasBuf
			n1n2Request.JsonData.N1MessageContainer = &n1MsgContainer
		}

		if n2Pdu, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			logger.SmfLog.Errorf("Build PDUSessionResourceSetupRequestTransfer failed: %s", err)
		} else {
			n1n2Request.BinaryDataN2Information = n2Pdu
			n1n2Request.JsonData.N2InfoContainer = &n2InfoContainer
		}
	} else {
		if smNasBuf, err := context.BuildGSMPDUSessionEstablishmentReject(smContext,
			nasMessage.Cause5GSMRequestRejectedUnspecified); err != nil {
			logger.SmfLog.Errorf("Build GSM PDUSessionEstablishmentReject failed: %s", err)
		} else {
			n1n2Request.BinaryDataN1Message = smNasBuf
			n1n2Request.JsonData.N1MessageContainer = &n1MsgContainer
		}
	}

	smContext.SubPduSessLog.Infof("N1N2 transfer initiated")
	// communicationClient := Namf_Communication.NewAPIClient(communicationConf)
	rspData, err := amf_producer.CreateN1N2MessageTransfer(smContext.Supi, n1n2Request, "")
	if err != nil {
		smContext.SubPfcpLog.Warnf("Send N1N2Transfer failed, %v ", err.Error())
		err = smContext.CommitSmPolicyDecision(false)
		if err != nil {
			smContext.SubPfcpLog.Errorf("CommitSmPolicyDecision failed, %v", err)
		}
		return err
	}
	if rspData.Cause == models.N1N2MessageTransferCause_N1_MSG_NOT_TRANSFERRED {
		smContext.SubPfcpLog.Errorf("N1N2MessageTransfer failure, %v", rspData.Cause)
		err = smContext.CommitSmPolicyDecision(false)
		if err != nil {
			smContext.SubPfcpLog.Errorf("CommitSmPolicyDecision failed, %v", err)
		}
		return fmt.Errorf("N1N2MessageTransfer failure, %v", rspData.Cause)
	}

	err = smContext.CommitSmPolicyDecision(true)
	if err != nil {
		smContext.SubPfcpLog.Errorf("CommitSmPolicyDecision failed, %v", err)
	}
	smContext.SubPduSessLog.Infof("Message content: %v", rspData)
	smContext.SubPduSessLog.Infof("N1N2 Transfer completed")
	return nil
}
