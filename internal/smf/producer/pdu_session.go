// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"fmt"

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

	createData := request.JSONData

	// Create SM context
	smContext.SetCreateData(createData)
	smContext.SmStatusNotifyURI = createData.SmContextStatusURI

	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	// DNN Information from config
	smContext.DNNInfo = context.RetrieveDnnInformation(*createData.SNssai, createData.Dnn)
	if smContext.DNNInfo == nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		return "", response, fmt.Errorf("couldn't find DNN information: snssai does not match DNN config: Sst: %d, Sd: %s, DNN: %s", createData.SNssai.Sst, createData.SNssai.Sd, createData.Dnn)
	}

	// IP Allocation
	smfSelf := context.SMFSelf()
	ip, err := smfSelf.DBInstance.AllocateIP(smContext.Supi)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMInsufficientResources)
		return "", response, fmt.Errorf("failed to allocate IP address: %v", err)
	}
	smContext.SubPduSessLog.Infof("Successfully allocated IP address: %s", smContext.PDUAddress.IP.String())
	smContext.PDUAddress = &context.UeIPAddr{IP: ip, UpfProvided: false}

	snssaiStr, err := marshtojsonstring.MarshToJSONString(createData.SNssai)
	if err != nil {
		return "", nil, fmt.Errorf("failed marshalling SNssai: %v", err)
	}

	snssai := snssaiStr[0]
	sessSubData, err := udm.GetAndSetSmData(smContext.Supi, createData.Dnn, snssai)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("failed to get subscription data: %v", err)
	}

	if len(sessSubData) == 0 {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("no subscription data")
	}

	smContext.DnnConfiguration = sessSubData[0].DnnConfigurations[createData.Dnn]

	// Decode UE content(PCO)
	establishmentRequest := m.PDUSessionEstablishmentRequest
	smContext.HandlePDUSessionEstablishmentRequest(establishmentRequest)

	// PCF Policy Association
	var smPolicyDecision *models.SmPolicyDecision
	smPolicyDecisionRsp, err := consumer.SendSMPolicyAssociationCreate(smContext)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("error creating policy association: %v", err)
	}
	smContext.SubPduSessLog.Infof("created policy association")
	smPolicyDecision = smPolicyDecisionRsp

	policyUpdates := qos.BuildSmPolicyUpdate(&smContext.SmPolicyData, smPolicyDecision)
	smContext.SmPolicyUpdates = append(smContext.SmPolicyUpdates, policyUpdates)

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
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("couldn't get default user plane path: %v", err)
	}
	defaultPath, err = context.GenerateDataPath(defaultUPPath, smContext)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("couldn't generate data path: %v", err)
	}
	if defaultPath != nil {
		defaultPath.IsDefaultPath = true
		smContext.Tunnel.AddDataPath(defaultPath)

		if err := defaultPath.ActivateTunnelAndPDR(smContext, 255); err != nil {
			response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
			return "", response, fmt.Errorf("couldn't activate data path: %v", err)
		}
	}
	if defaultPath == nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMInsufficientResourcesForSpecificSliceAndDNN)
		return "", response, fmt.Errorf("default data path not found")
	}

	_ = smContext.BuildCreatedData()

	smContext.SubPduSessLog.Infof("successfully created PDU session context")

	return smContext.Ref, nil, nil
}

func HandlePDUSessionSMContextUpdate(request models.UpdateSmContextRequest, smContext *context.SMContext) (*models.UpdateSmContextResponse, error) {
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	pfcpAction := &pfcpAction{}
	var response models.UpdateSmContextResponse
	response.JSONData = new(models.SmContextUpdatedData)

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

	// Initiate PFCP Release
	if pfcpAction.sendPfcpDelete {
		if err = SendPfcpSessionReleaseReq(smContext); err != nil {
			return nil, fmt.Errorf("pfcp session release error: %v ", err.Error())
		}
	} else if pfcpAction.sendPfcpModify {
		smContext.SubPduSessLog.Infof("PDUSessionSMContextUpdate, send PFCP Modification")

		// Initiate PFCP Modify
		err := SendPfcpSessionModifyReq(smContext, pfcpParam)
		if err != nil {
			return nil, fmt.Errorf("pfcp session modify error: %v ", err.Error())
		}
	}

	return &response, nil
}

func HandlePDUSessionSMContextRelease(smContext *context.SMContext) error {
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	// Send Policy delete
	err := consumer.SendSMPolicyAssociationDelete(smContext.Supi, smContext.PDUSessionID)
	if err != nil {
		smContext.SubCtxLog.Errorf("error deleting policy association: %v", err)
	}

	// Release UE IP-Address
	err = smContext.ReleaseUeIPAddr()
	if err != nil {
		smContext.SubPduSessLog.Errorf("release UE IP address failed: %v", err)
	}

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
		context.RemoveSMContext(smContext.Ref)
		return nil

	case context.SessionReleaseTimeout:
		context.RemoveSMContext(smContext.Ref)
		return fmt.Errorf("PFCP session release timeout")

	case context.SessionReleaseFailed:
		context.RemoveSMContext(smContext.Ref)
		return fmt.Errorf("PFCP session release failed")

	default:
		smContext.SubCtxLog.Warnf("PDUSessionSMContextRelease, The state shouldn't be [%s]\n", status)
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
		N2InformationClass: models.N2InformationClassSM,
		SmInfo: &models.N2SmInformation{
			PduSessionID: smContext.PDUSessionID,
			N2InfoContent: &models.N2InfoContent{
				NgapIeType: models.NgapIeTypePduResSetupReq,
				NgapData: &models.RefToBinaryData{
					ContentID: "N2SmInformation",
				},
			},
			SNssai: smContext.Snssai,
		},
	}

	// N1 Container Info
	n1MsgContainer := models.N1MessageContainer{
		N1MessageClass:   "SM",
		N1MessageContent: &models.RefToBinaryData{ContentID: "GSM_NAS"},
	}

	// N1N2 Json Data
	n1n2Request.JSONData = &models.N1N2MessageTransferReqData{PduSessionID: smContext.PDUSessionID}

	if success {
		if smNasBuf, err := context.BuildGSMPDUSessionEstablishmentAccept(smContext); err != nil {
			logger.SmfLog.Errorf("Build GSM PDUSessionEstablishmentAccept failed: %s", err)
		} else {
			n1n2Request.BinaryDataN1Message = smNasBuf
			n1n2Request.JSONData.N1MessageContainer = &n1MsgContainer
		}

		if n2Pdu, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
			logger.SmfLog.Errorf("Build PDUSessionResourceSetupRequestTransfer failed: %s", err)
		} else {
			n1n2Request.BinaryDataN2Information = n2Pdu
			n1n2Request.JSONData.N2InfoContainer = &n2InfoContainer
		}
	} else {
		if smNasBuf, err := context.BuildGSMPDUSessionEstablishmentReject(smContext,
			nasMessage.Cause5GSMRequestRejectedUnspecified); err != nil {
			logger.SmfLog.Errorf("Build GSM PDUSessionEstablishmentReject failed: %s", err)
		} else {
			n1n2Request.BinaryDataN1Message = smNasBuf
			n1n2Request.JSONData.N1MessageContainer = &n1MsgContainer
		}
	}

	rspData, err := amf_producer.CreateN1N2MessageTransfer(smContext.Supi, n1n2Request, "")
	if err != nil {
		err = smContext.CommitSmPolicyDecision(false)
		if err != nil {
			return fmt.Errorf("failed to commit sm policy decision: %v", err)
		}
		return fmt.Errorf("failed to send n1 n2 transfer request: %v", err)
	}
	smContext.SubPduSessLog.Infof("sent n1 n2 transfer request")
	if rspData.Cause == models.N1N2MessageTransferCauseN1MsgNotTransferred {
		err = smContext.CommitSmPolicyDecision(false)
		if err != nil {
			return fmt.Errorf("failed to commit sm policy decision: %v", err)
		}
		return fmt.Errorf("failed to send n1 n2 transfer request: %v", rspData.Cause)
	}

	err = smContext.CommitSmPolicyDecision(true)
	if err != nil {
		return fmt.Errorf("failed to commit sm policy decision: %v", err)
	}
	return nil
}
