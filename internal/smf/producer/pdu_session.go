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
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/util/marshtojsonstring"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

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

func HandlePDUSessionSMContextCreate(request models.PostSmContextsRequest, smContext *context.SMContext) (string, *models.PostSmContextsErrorResponse, error) {
	// GSM State
	// PDU Session Establishment Accept/Reject

	// Check has PDU Session Establishment Request
	m := nas.NewMessage()
	if err := m.GsmMessageDecode(&request.BinaryDataN1SmMessage); err != nil ||
		m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		errRsp := &models.PostSmContextsErrorResponse{}
		return "", errRsp, fmt.Errorf("error decoding NAS message: %v", err)
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
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		return "", response, fmt.Errorf("SnssaiError")
	}

	// IP Allocation
	smfSelf := context.SMF_Self()
	if ip, err := smfSelf.DbInstance.AllocateIP(smContext.Supi); err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, failed allocate IP address: ", err)
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMInsufficientResources)
		return "", response, fmt.Errorf("failed allocate IP address: %v", err)
	} else {
		smContext.PDUAddress = &context.UeIpAddr{Ip: ip, UpfProvided: false}
		smContext.SubPduSessLog.Infof("Successful IP Allocation: %s", smContext.PDUAddress.Ip.String())
	}

	snssai := marshtojsonstring.MarshToJsonString(createData.SNssai)[0]

	sessSubData, err := udm.GetAndSetSmData(smContext.Supi, createData.Dnn, snssai)
	if err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, get SessionManagementSubscriptionData error: ", err)
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("SubscriptionError")
	}
	if len(sessSubData) > 0 {
		smContext.DnnConfiguration = sessSubData[0].DnnConfigurations[createData.Dnn]
		smContext.SubPduSessLog.Infof("subscription data retrieved from UDM")
	} else {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SessionManagementSubscriptionData from UDM is nil")
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("NoSubscriptionError")
	}

	// Decode UE content(PCO)
	establishmentRequest := m.PDUSessionEstablishmentRequest
	smContext.HandlePDUSessionEstablishmentRequest(establishmentRequest)

	smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, send NF Discovery Serving PCF success")

	// PCF Policy Association
	var smPolicyDecision *models.SmPolicyDecision
	if smPolicyDecisionRsp, httpStatus, err := consumer.SendSMPolicyAssociationCreate(smContext); err != nil {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SMPolicyAssociationCreate error: ", err)
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("PcfAssoError")
	} else if httpStatus != http.StatusCreated {
		smContext.SubPduSessLog.Errorln("PDUSessionSMContextCreate, SMPolicyAssociationCreate http status: ", http.StatusText(httpStatus))
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("PcfAssoError")
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
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("DataPathError")
	}
	defaultPath, err = context.GenerateDataPath(defaultUPPath, smContext)
	if err != nil {
		smContext.SubPduSessLog.Errorf("couldn't generate data path: %v", err.Error())
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("DataPathError")
	}
	if defaultPath != nil {
		defaultPath.IsDefaultPath = true
		smContext.Tunnel.AddDataPath(defaultPath)

		if err := defaultPath.ActivateTunnelAndPDR(smContext, 255); err != nil {
			smContext.SubPduSessLog.Errorf("PDUSessionSMContextCreate, data path error: %v", err.Error())
			response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
			return "", response, fmt.Errorf("DataPathError")
		}
	}
	if defaultPath == nil {
		smContext.ChangeState(context.SmStateInit)
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMInsufficientResourcesForSpecificSliceAndDNN)
		return "", response, fmt.Errorf("default data path not found")
	}

	_ = smContext.BuildCreatedData()

	smContext.SubPduSessLog.Infof("PDUSessionSMContextCreate, PDU session context create success ")

	return smContext.Ref, nil, nil
}

func HandlePDUSessionSMContextUpdate(request models.UpdateSmContextRequest, smContext *context.SMContext) (*models.UpdateSmContextResponse, error) {
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	pfcpAction := &pfcpAction{}
	var response models.UpdateSmContextResponse
	response.JsonData = new(models.SmContextUpdatedData)

	err := HandleUpdateN1Msg(request, smContext, &response, pfcpAction)
	if err != nil {
		return nil, err
	}

	pfcpParam := &pfcpParam{
		pdrList: []*context.PDR{},
		farList: []*context.FAR{},
		barList: []*context.BAR{},
		qerList: []*context.QER{},
	}

	// UP Cnx State handling
	if err := HandleUpCnxState(request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return nil, err
	}

	// N2 Msg Handling
	if err := HandleUpdateN2Msg(request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return nil, err
	}

	// Ho state handling
	if err := HandleUpdateHoState(request, smContext, &response); err != nil {
		return nil, err
	}

	// Cause handling
	if err := HandleUpdateCause(request, smContext, &response, pfcpAction); err != nil {
		return nil, err
	}

	var returnErr error
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
		} else if pfcpAction.sendPfcpModify {
			smContext.ChangeState(context.SmStatePfcpModify)
			smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send PFCP Modification")

			// Initiate PFCP Modify
			if err = SendPfcpSessionModifyReq(smContext, pfcpParam); err != nil {
				// Modify failure
				smContext.SubCtxLog.Errorf("pfcp session modify error: %v ", err.Error())

				// Form Modify err rsp
				returnErr = fmt.Errorf("pfcp session modify error: %v ", err.Error())
			} else {
				smContext.ChangeState(context.SmStateActive)
			}
		}

	case context.SmStateModify:
		smContext.ChangeState(context.SmStateActive)

	case context.SmStateInit, context.SmStateInActivePending:

	default:
		smContext.SubPduSessLog.Warnf("PDUSessionSMContextUpdate, SM Context State [%s] shouldn't be here\n", smContext.SMContextState)
	}

	return &response, returnErr
}

func HandlePDUSessionSMContextRelease(smContext *context.SMContext) error {
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	// Send Policy delete
	if httpStatus, err := consumer.SendSMPolicyAssociationDelete(smContext); err != nil {
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

	// Release User-plane
	status, ok := releaseTunnel(smContext)
	if !ok {
		context.RemoveSMContext(smContext.Ref)
		logger.SmfLog.Warnf("sm context was already released: %s", smContext.Ref)
		return nil
	}
	// var releaseErr error
	switch *status {
	case context.SessionReleaseSuccess:
		smContext.ChangeState(context.SmStatePfcpRelease)
		context.RemoveSMContext(smContext.Ref)
		return nil

	case context.SessionReleaseTimeout:
		smContext.ChangeState(context.SmStateActive)
		context.RemoveSMContext(smContext.Ref)
		return fmt.Errorf("PFCP session release timeout")

	case context.SessionReleaseFailed:
		smContext.ChangeState(context.SmStateActive)
		context.RemoveSMContext(smContext.Ref)
		return fmt.Errorf("PFCP session release failed")

	default:
		smContext.SubCtxLog.Warnf("PDUSessionSMContextRelease, The state shouldn't be [%s]\n", status)
		smContext.ChangeState(context.SmStateActive)
		context.RemoveSMContext(smContext.Ref)
		return fmt.Errorf("PFCP session release failed: unknown status")
	}
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
