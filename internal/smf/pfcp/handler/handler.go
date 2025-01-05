// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"fmt"
	"net"

	amf_producer "github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/logger"
	smf_context "github.com/ellanetworks/core/internal/smf/context"
	pfcp_message "github.com/ellanetworks/core/internal/smf/pfcp/message"
	"github.com/ellanetworks/core/internal/smf/pfcp/udp"
	"github.com/ellanetworks/core/internal/smf/producer"
	"github.com/omec-project/openapi/models"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func FindFTEID(createdPDRIEs []*ie.IE) (*ie.FTEIDFields, error) {
	for _, createdPDRIE := range createdPDRIEs {
		teid, err := createdPDRIE.FTEID()
		if err == nil {
			return teid, nil
		}
	}
	return nil, fmt.Errorf("FTEID not found in CreatedPDR")
}

func FindUEIPAddress(createdPDRIEs []*ie.IE) net.IP {
	for _, createdPDRIE := range createdPDRIEs {
		ueIPAddress, err := createdPDRIE.UEIPAddress()
		if err == nil {
			return ueIPAddress.IPv4Address
		}
	}
	return nil
}

func HandlePfcpVersionNotSupportedResponse(msg *udp.Message) {
	logger.SmfLog.Warnf("PFCP Version Not Support Response handling is not implemented")
}

func HandlePfcpNodeReportRequest(msg *udp.Message) {
	logger.SmfLog.Warnf("PFCP Node Report Request handling is not implemented")
}

func HandlePfcpNodeReportResponse(msg *udp.Message) {
	logger.SmfLog.Warnf("PFCP Node Report Response handling is not implemented")
}

func HandlePfcpSessionSetDeletionRequest(msg *udp.Message) {
	logger.SmfLog.Warnf("PFCP Session Set Deletion Request handling is not implemented")
}

func HandlePfcpSessionSetDeletionResponse(msg *udp.Message) {
	logger.SmfLog.Warnf("PFCP Session Set Deletion Response handling is not implemented")
}

func HandlePfcpSessionEstablishmentResponse(msg *udp.Message) {
	rsp, ok := msg.PfcpMessage.(*message.SessionEstablishmentResponse)
	if !ok {
		logger.SmfLog.Errorf("invalid message type for session establishment response")
		return
	}
	logger.SmfLog.Infof("handle PFCP Session Establishment Response")

	SEID := rsp.SEID()
	if SEID == 0 {
		if eventData, ok := msg.EventData.(udp.PfcpEventData); !ok {
			logger.SmfLog.Warnf("PFCP Session Establish Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := smf_context.GetSMContextBySEID(SEID)
	if smContext == nil {
		logger.SmfLog.Errorf("failed to find SMContext for SEID[%d]", SEID)
		return
	}
	smContext.SMLock.Lock()

	// Get NodeId from Seq:NodeId Map
	seq := rsp.Sequence()
	nodeID := pfcp_message.FetchPfcpTxn(seq)

	if rsp.UPFSEID != nil {
		// NodeIDtoIP := rsp.NodeID.ResolveNodeIdToIp().String()
		NodeIDtoIP := nodeID.ResolveNodeIdToIp().String()
		pfcpSessionCtx := smContext.PFCPContext[NodeIDtoIP]
		rspUPFseid, err := rsp.UPFSEID.FSEID()
		if err != nil {
			logger.SmfLog.Errorf("failed to parse FSEID IE: %+v", err)
			return
		}
		pfcpSessionCtx.RemoteSEID = rspUPFseid.SEID
		smContext.SubPfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse rsp.UPFSEID.Seid [%v] ", rspUPFseid.SEID)
	}

	// Get N3 interface UPF
	ANUPF := smContext.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode

	// UE IP-Addr(only v4 supported)
	if rsp.CreatedPDR != nil {
		ueIPAddress := FindUEIPAddress(rsp.CreatedPDR)
		if ueIPAddress != nil {
			smContext.SubPfcpLog.Infof("upf provided ue ip address [%v]", ueIPAddress)
			// Release previous locally allocated UE IP-Addr
			err := smContext.ReleaseUeIpAddr()
			if err != nil {
				logger.SmfLog.Errorf("failed to release UE IP-Addr: %+v", err)
			}

			// Update with one received from UPF
			smContext.PDUAddress.Ip = ueIPAddress
			smContext.PDUAddress.UpfProvided = true
		}

		// Store F-TEID created by UPF
		fteid, err := FindFTEID(rsp.CreatedPDR)
		if err != nil {
			logger.SmfLog.Errorf("failed to parse TEID IE: %+v", err)
			return
		}
		logger.SmfLog.Infof("created PDR FTEID: %+v", fteid)
		ANUPF.UpLinkTunnel.TEID = fteid.TEID
		upf := smf_context.GetUserPlaneInformation().UPF.UPF
		if upf == nil {
			logger.SmfLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
			return
		}
		upf.N3Interfaces = make([]smf_context.UPFInterfaceInfo, 0)
		n3Interface := smf_context.UPFInterfaceInfo{}
		n3Interface.IPv4EndPointAddresses = append(n3Interface.IPv4EndPointAddresses, fteid.IPv4Address)
		upf.N3Interfaces = append(upf.N3Interfaces, n3Interface)
	}
	smContext.SMLock.Unlock()

	if rsp.NodeID == nil {
		logger.SmfLog.Errorf("PFCP Session Establishment Response missing NodeID")
		return
	}
	rspNodeIDStr, err := rsp.NodeID.NodeID()
	if err != nil {
		logger.SmfLog.Errorf("failed to parse NodeID IE: %+v", err)
		return
	}
	rspNodeID := smf_context.NewNodeID(rspNodeIDStr)

	if ANUPF.UPF.NodeID.ResolveNodeIdToIp().Equal(nodeID.ResolveNodeIdToIp()) {
		// UPF Accept
		if rsp.Cause == nil {
			logger.SmfLog.Errorf("PFCP Session Establishment Response missing Cause")
			return
		}
		causeValue, err := rsp.Cause.Cause()
		if err != nil {
			logger.SmfLog.Errorf("failed to parse Cause IE: %+v", err)
			return
		}
		if causeValue == ie.CauseRequestAccepted {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionEstablishSuccess
			smContext.SubPfcpLog.Infof("PFCP Session Establishment accepted")
		} else {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionEstablishFailed
			smContext.SubPfcpLog.Errorf("PFCP Session Establishment rejected with cause [%v]", causeValue)
		}
	}

	if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == smf_context.AddingPSA {
			smContext.SubPfcpLog.Infoln("keep Adding PSAndULCL")
			producer.AddPDUSessionAnchorAndULCL(smContext, *rspNodeID)
			smContext.BPManager.BPStatus = smf_context.AddingPSA
		}
	}
}

func HandlePfcpSessionModificationResponse(msg *udp.Message) {
	rsp, ok := msg.PfcpMessage.(*message.SessionModificationResponse)
	if !ok {
		logger.SmfLog.Errorf("invalid message type for session establishment response")
		return
	}

	logger.SmfLog.Infof("handle PFCP Session Modification Response")

	SEID := rsp.SEID()

	if SEID == 0 {
		if eventData, ok := msg.EventData.(udp.PfcpEventData); !ok {
			logger.SmfLog.Warnf("PFCP Session Modification Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := smf_context.GetSMContextBySEID(SEID)

	logger.SmfLog.Infoln("in HandlePfcpSessionModificationResponse")

	if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == smf_context.AddingPSA {
			smContext.SubPfcpLog.Infoln("keep Adding PSAAndULCL")

			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			producer.AddPDUSessionAnchorAndULCL(smContext, upfNodeID)
		}
	}

	if rsp.Cause == nil {
		logger.SmfLog.Errorf("PFCP Session Modification Response missing Cause")
		return
	}

	causeValue, err := rsp.Cause.Cause()
	if err != nil {
		logger.SmfLog.Errorf("failed to parse Cause IE: %+v", err)
		return
	}

	if causeValue == ie.CauseRequestAccepted {
		smContext.SubPduSessLog.Infoln("PFCP Modification Response Accept")
		if smContext.SMContextState == smf_context.SmStatePfcpModify {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Debugf("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() {
				smContext.SBIPFCPCommunicationChan <- smf_context.SessionUpdateSuccess
			}

			if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
				if smContext.BPManager.BPStatus == smf_context.UnInitialized {
					smContext.SubPfcpLog.Infoln("add PSAAndULCL")
					upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
					producer.AddPDUSessionAnchorAndULCL(smContext, upfNodeID)
					smContext.BPManager.BPStatus = smf_context.AddingPSA
				}
			}
		}

		smContext.SubPfcpLog.Infof("PFCP Session Modification Success[%d]\n", SEID)
	} else {
		smContext.SubPfcpLog.Infof("PFCP Session Modification Failed[%d]\n", SEID)
		if smContext.SMContextState == smf_context.SmStatePfcpModify {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionUpdateFailed
		}
	}

	smContext.SubCtxLog.Debugln("PFCP Session Context")
	for _, ctx := range smContext.PFCPContext {
		smContext.SubCtxLog.Debugln(ctx.String())
	}
}

func HandlePfcpSessionDeletionResponse(msg *udp.Message) {
	rsp, ok := msg.PfcpMessage.(*message.SessionDeletionResponse)
	if !ok {
		logger.SmfLog.Errorf("invalid message type for session deletion response")
		return
	}
	logger.SmfLog.Infof("handle PFCP Session Deletion Response")
	SEID := rsp.SEID()

	if SEID == 0 {
		if eventData, ok := msg.EventData.(udp.PfcpEventData); !ok {
			logger.SmfLog.Warnf("PFCP Session Deletion Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := smf_context.GetSMContextBySEID(SEID)

	if smContext == nil {
		logger.SmfLog.Warnf("PFCP Session Deletion Response found SM context nil, response discarded")
		return
	}

	if rsp.Cause == nil {
		logger.SmfLog.Errorf("PFCP Session Deletion Response missing Cause")
		return
	}

	causeValue, err := rsp.Cause.Cause()
	if err != nil {
		logger.SmfLog.Errorf("failed to parse Cause IE: %+v", err)
		return
	}

	if causeValue == ie.CauseRequestAccepted {
		if smContext.SMContextState == smf_context.SmStatePfcpRelease {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Debugf("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() && !smContext.LocalPurged {
				smContext.SBIPFCPCommunicationChan <- smf_context.SessionReleaseSuccess
			}
		}
		smContext.SubPfcpLog.Infof("PFCP Session Deletion Success[%d]\n", SEID)
	} else {
		if smContext.SMContextState == smf_context.SmStatePfcpRelease && !smContext.LocalPurged {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionReleaseSuccess
		}
		smContext.SubPfcpLog.Infof("PFCP Session Deletion Failed[%d]\n", SEID)
	}
}

func HandlePfcpSessionReportRequest(msg *udp.Message) {
	req, ok := msg.PfcpMessage.(*message.SessionReportRequest)
	if !ok {
		logger.SmfLog.Errorf("invalid message type for session report request")
		return
	}

	logger.SmfLog.Infof("handle PFCP Session Report Request")

	SEID := req.SEID()
	smContext := smf_context.GetSMContextBySEID(SEID)
	seqFromUPF := req.Sequence()

	var cause uint8
	var pfcpSRflag smf_context.PFCPSRRspFlags

	if smContext == nil {
		logger.SmfLog.Warnf("PFCP Session Report Request Found SM Context NULL, Request Rejected")
		cause = ie.CauseRequestRejected

		// Rejecting buffering at UPF since not able to process Session Report Request
		pfcpSRflag.Drobu = true
		err := pfcp_message.SendPfcpSessionReportResponse(msg.RemoteAddr, cause, pfcpSRflag, seqFromUPF, SEID)
		if err != nil {
			logger.SmfLog.Errorf("failed to send PFCP Session Report Response: %+v", err)
		}
		return
	}

	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	if smContext.UpCnxState == models.UpCnxState_DEACTIVATED {
		if req.ReportType.HasDLDR() {
			downlinkServiceInfo, err := req.DownlinkDataReport.DownlinkDataServiceInformation()
			if err != nil {
				logger.SmfLog.Warnf("DownlinkDataServiceInformation not found in DownlinkDataReport")
			}

			if downlinkServiceInfo != nil {
				smContext.SubPfcpLog.Warnf("PFCP Session Report Request DownlinkDataServiceInformation handling is not implemented")
			}

			n1n2Request := models.N1N2MessageTransferRequest{}

			// TS 23.502 4.2.3.3 3a. Send Namf_Communication_N1N2MessageTransfer Request, SMF->AMF
			n2SmBuf, err := smf_context.BuildPDUSessionResourceSetupRequestTransfer(smContext)
			if err != nil {
				smContext.SubPduSessLog.Errorln("Build PDUSessionResourceSetupRequestTransfer failed:", err)
			} else {
				n1n2Request.BinaryDataN2Information = n2SmBuf
			}

			// n1n2FailureTxfNotifURI to be added in n1n2 request transfer.
			// It is used as path by AMF to send failure notification message towards SMF
			n1n2FailureTxfNotifURI := "/nsmf-callback/sm-n1n2failnotify/"
			n1n2FailureTxfNotifURI += smContext.Ref

			n1n2Request.JsonData = &models.N1N2MessageTransferReqData{
				PduSessionId:           smContext.PDUSessionID,
				SkipInd:                false,
				N1n2FailureTxfNotifURI: n1n2FailureTxfNotifURI,
				N2InfoContainer: &models.N2InfoContainer{
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
				},
			}
			// communicationClient := Namf_Communication.NewAPIClient(communicationConf)
			rspData, err := amf_producer.CreateN1N2MessageTransfer(smContext.Supi, n1n2Request, "")
			// rspData, _, err := communicationClient.
			// 	N1N2MessageCollectionDocumentApi.
			// 	N1N2MessageTransfer(context.Background(), smContext.Supi, n1n2Request)
			if err != nil {
				smContext.SubPfcpLog.Warnf("Send N1N2Transfer failed")
			}
			if rspData.Cause == models.N1N2MessageTransferCause_ATTEMPTING_TO_REACH_UE {
				smContext.SubPfcpLog.Infof("Receive %v, AMF is able to page the UE", rspData.Cause)

				pfcpSRflag.Drobu = false
				cause = ie.CauseRequestAccepted
			}
			if rspData.Cause == models.N1N2MessageTransferCause_UE_NOT_RESPONDING {
				smContext.SubPfcpLog.Infof("Receive %v, UE is not responding to N1N2 transfer message", rspData.Cause)

				// Adding Session report flag to drop buffered packet at UPF
				pfcpSRflag.Drobu = true

				// Adding Cause rejected since N1N2 Transfer message got rejected.
				cause = ie.CauseRequestRejected
			}

			// Sending Session Report Response to UPF.
			smContext.SubPfcpLog.Infof("Sending Session Report to UPF with Cause %v", cause)
			err = pfcp_message.SendPfcpSessionReportResponse(msg.RemoteAddr, cause, pfcpSRflag, seqFromUPF, SEID)
			if err != nil {
				logger.SmfLog.Errorf("failed to send PFCP Session Report Response: %+v", err)
			}
		}
	}
}

func HandlePfcpSessionReportResponse(msg *udp.Message) {
	logger.SmfLog.Warnf("PFCP Session Report Response handling is not implemented")
}
