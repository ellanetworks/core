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
	if smCtxt == nil {
		return nil
	}

	smCtxt.SMLock.Lock()
	context.RemoveSMContext(smCtxt.Ref)

	// Check if UPF session set, send release
	if smCtxt.Tunnel != nil {
		err := releaseTunnel(smCtxt)
		if err != nil {
			smCtxt.SubPduSessLog.Errorf("release tunnel failed: %v", err)
		}
	}

	smCtxt.SMLock.Unlock()

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
	dnnInfo, err := context.RetrieveDnnInformation(*createData.SNssai, createData.Dnn)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("failed to retrieve DNN information: %v", err)
	}
	if dnnInfo == nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		return "", response, fmt.Errorf("couldn't find DNN information: snssai does not match DNN config: Sst: %d, Sd: %s, DNN: %s", createData.SNssai.Sst, createData.SNssai.Sd, createData.Dnn)
	}
	smContext.DNNInfo = dnnInfo

	// IP Allocation
	smfSelf := context.SMFSelf()
	ip, err := smfSelf.DBInstance.AllocateIP(smContext.Supi)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMInsufficientResources)
		return "", response, fmt.Errorf("failed to allocate IP address: %v", err)
	}
	smContext.SubPduSessLog.Infof("Successfully allocated IP address: %s", ip.String())
	smContext.PDUAddress = &context.UeIPAddr{IP: ip}

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
	smPolicyDecisionRsp, err := SendSMPolicyAssociationCreate(smContext)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("error creating policy association: %v", err)
	}
	smContext.SubPduSessLog.Infof("Created policy association")
	smPolicyDecision = smPolicyDecisionRsp

	policyUpdates := qos.BuildSmPolicyUpdate(&smContext.SmPolicyData, smPolicyDecision)
	smContext.SmPolicyUpdates = append(smContext.SmPolicyUpdates, policyUpdates)

	defaultPath := context.GenerateDataPath(smfSelf.UPF, smContext)
	smContext.Tunnel = &context.UPTunnel{
		DataPath: defaultPath,
	}

	err = defaultPath.ActivateTunnelAndPDR(smContext, 255)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", response, fmt.Errorf("couldn't activate data path: %v", err)
	}

	_ = smContext.BuildCreatedData()

	smContext.SubPduSessLog.Infof("Successfully created PDU session context")

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
		return nil, fmt.Errorf("error handling N1 message: %v", err)
	}

	pfcpParam := &pfcpParam{
		pdrList: []*context.PDR{},
		farList: []*context.FAR{},
		barList: []*context.BAR{},
		qerList: []*context.QER{},
	}

	// UP Cnx State handling
	if err := HandleUpCnxState(request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return nil, fmt.Errorf("error handling UP connection state: %v", err)
	}

	// N2 Msg Handling
	if err := HandleUpdateN2Msg(request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	// Ho state handling
	if err := HandleUpdateHoState(request, smContext, &response); err != nil {
		return nil, fmt.Errorf("error handling HO state: %v", err)
	}

	// Cause handling
	if err := HandleUpdateCause(request, smContext, &response, pfcpAction); err != nil {
		return nil, fmt.Errorf("error handling cause: %v", err)
	}

	// Initiate PFCP Release
	if pfcpAction.sendPfcpDelete {
		err = SendPfcpSessionReleaseReq(smContext)
		if err != nil {
			return nil, fmt.Errorf("pfcp session release error: %v ", err.Error())
		}
		smContext.SubPduSessLog.Infof("Sent PFCP session release request")
	} else if pfcpAction.sendPfcpModify {
		err := SendPfcpSessionModifyReq(smContext, pfcpParam)
		if err != nil {
			return nil, fmt.Errorf("pfcp session modify error: %v ", err.Error())
		}
		smContext.SubPduSessLog.Infof("Sent PFCP session modification request")
	}

	return &response, nil
}

func HandlePDUSessionSMContextRelease(smContext *context.SMContext) error {
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	// Send Policy delete
	err := SendSMPolicyAssociationDelete(smContext.Supi, smContext.PDUSessionID)
	if err != nil {
		smContext.SubCtxLog.Errorf("error deleting policy association: %v", err)
	}

	// Release UE IP-Address
	err = smContext.ReleaseUeIPAddr()
	if err != nil {
		smContext.SubPduSessLog.Errorf("release UE IP address failed: %v", err)
	}

	// Release User-plane
	err = releaseTunnel(smContext)
	if err != nil {
		context.RemoveSMContext(smContext.Ref)
		return fmt.Errorf("release tunnel failed: %v", err)
	}
	context.RemoveSMContext(smContext.Ref)
	return nil
}

func releaseTunnel(smContext *context.SMContext) error {
	if smContext.Tunnel == nil {
		return fmt.Errorf("tunnel not found")
	}
	deletedPFCPNode := make(map[string]bool)
	dataPath := smContext.Tunnel.DataPath
	smContext.Tunnel.DataPath.DeactivateTunnelAndPDR(smContext)
	curDataPathNode := dataPath.DPNode
	curUPFID := curDataPathNode.UPF.NodeID.String()
	if _, exist := deletedPFCPNode[curUPFID]; !exist {
		err := pfcp.SendPfcpSessionDeletionRequest(curDataPathNode.UPF.NodeID, smContext)
		if err != nil {
			return fmt.Errorf("send PFCP session deletion request failed: %v", err)
		}
		deletedPFCPNode[curUPFID] = true
	}
	smContext.Tunnel = nil
	return nil
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
	smContext.SubPduSessLog.Infof("Sent n1 n2 transfer request")
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
