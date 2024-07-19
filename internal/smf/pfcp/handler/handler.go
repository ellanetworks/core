package handler

import (
	"context"
	"fmt"
	"net"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpType"
	"github.com/omec-project/pfcp/pfcpUdp"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
	pfcp_message "github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/producer"
)

func HandlePfcpHeartbeatRequest(msg *pfcpUdp.Message) {
	h := msg.PfcpMessage.Header
	pfcp_message.SendHeartbeatResponse(msg.RemoteAddr, h.SequenceNumber)
}

func HandlePfcpHeartbeatResponse(msg *pfcpUdp.Message) {
	rsp := msg.PfcpMessage.Body.(pfcp.HeartbeatResponse)

	logger.PfcpLog.Infof("Handle pfcp heartbeat response")

	// Get NodeId from Seq:NodeId Map
	seq := msg.PfcpMessage.Header.SequenceNumber
	nodeID := pfcp_message.FetchPfcpTxn(seq)

	if nodeID == nil {
		logger.PfcpLog.Errorf("No pending pfcp heartbeat response for sequence no: %v", seq)
		return
	}

	logger.PfcpLog.Debugf("handle pfcp heartbeat response seq[%d] with NodeID[%v, %s]", seq, nodeID, nodeID.ResolveNodeIdToIp().String())

	upf := smf_context.RetrieveUPFNodeByNodeID(*nodeID)
	if upf == nil {
		logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
		return
	}
	upf.UpfLock.Lock()
	defer upf.UpfLock.Unlock()

	if *rsp.RecoveryTimeStamp != upf.RecoveryTimeStamp {
		// change UPF state to not associated so that
		// PFCP Association can be initiated again
		upf.UPFStatus = smf_context.NotAssociated
		logger.PfcpLog.Warnf("PFCP Heartbeat Response, upf [%v] recovery timestamp changed, previous [%v], new [%v] ", upf.NodeID, upf.RecoveryTimeStamp, *rsp.RecoveryTimeStamp)

		// TODO: Session cleanup required and updated to AMF/PCF
	}

	upf.NHeartBeat = 0 // reset Heartbeat attempt to 0
}

func SetUpfInactive(nodeID pfcpType.NodeID, msgType pfcp.MessageType) {
	upf := smf_context.RetrieveUPFNodeByNodeID(nodeID)
	if upf == nil {
		logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
		return
	}

	upf.UpfLock.Lock()
	defer upf.UpfLock.Unlock()
	upf.UPFStatus = smf_context.NotAssociated
	upf.NHeartBeat = 0 // reset Heartbeat attempt to 0
}

func HandlePfcpAssociationSetupRequest(msg *pfcpUdp.Message) {
	req := msg.PfcpMessage.Body.(pfcp.PFCPAssociationSetupRequest)

	logger.PfcpLog.Infof("Handle PFCP Association Setup Request")

	if msg.RemoteAddr.Port != 8806 {
		logger.PfcpLog.Errorf("Received PFCP Association Setup Request from non-UPF, dropping")
		return
	}
	nodeID := req.NodeID
	if nodeID == nil {
		logger.PfcpLog.Errorln("pfcp association needs NodeID")
		return
	}

	logger.PfcpLog.Infof("Handle PFCP Association Setup Request with NodeID[%s]", nodeID.ResolveNodeIdToIp().String())

	upf := smf_context.RetrieveUPFNodeByNodeID(*nodeID)
	if upf == nil {
		logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
		return
	}

	upf.UpfLock.Lock()
	defer upf.UpfLock.Unlock()
	upf.UPIPInfo = *req.UserPlaneIPResourceInformation
	upf.RecoveryTimeStamp = *req.RecoveryTimeStamp
	upf.NHeartBeat = 0 // reset Heartbeat attempt to 0

	// Response with PFCP Association Setup Response
	cause := pfcpType.Cause{
		CauseValue: pfcpType.CauseRequestAccepted,
	}
	pfcp_message.SendPfcpAssociationSetupResponse(*nodeID, cause, upf.Port)
}

func HandlePfcpAssociationSetupResponse(msg *pfcpUdp.Message) {
	rsp := msg.PfcpMessage.Body.(pfcp.PFCPAssociationSetupResponse)

	nodeID := rsp.NodeID
	if rsp.Cause.CauseValue == pfcpType.CauseRequestAccepted {
		if nodeID == nil {
			logger.PfcpLog.Errorln("pfcp association needs NodeID")
			return
		}
		logger.PfcpLog.Infof("Handle PFCP Association Setup Response with NodeID[%s]", nodeID.ResolveNodeIdToIp().String())

		// Get NodeId from Seq:NodeId Map
		seq := msg.PfcpMessage.Header.SequenceNumber
		nodeID = pfcp_message.FetchPfcpTxn(seq)

		if nodeID == nil {
			logger.PfcpLog.Errorf("No pending pfcp Assoc req for sequence no: %v", seq)
			return
		}

		upf := smf_context.RetrieveUPFNodeByNodeID(*nodeID)
		if upf == nil {
			logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
			return
		}

		// validate if DNNs served by UPF matches with the one provided by UPF
		if rsp.UserPlaneIPResourceInformation != nil {
			upfProvidedDnn := string(rsp.UserPlaneIPResourceInformation.NetworkInstance)
			if !upf.IsDnnConfigured(upfProvidedDnn) {
				logger.PfcpLog.Errorf("Handle PFCP Association Setup success Response, DNN mismatch, [%v] is not configured ", upfProvidedDnn)
				return
			}
		}

		upf.UpfLock.Lock()
		defer upf.UpfLock.Unlock()
		upf.UPFStatus = smf_context.AssociatedSetUpSuccess
		upf.RecoveryTimeStamp = *rsp.RecoveryTimeStamp
		upf.NHeartBeat = 0 // reset Heartbeat attempt to 0

		// Supported Features of UPF
		if rsp.UPFunctionFeatures != nil {
			logger.PfcpLog.Debugf("Handle PFCP Association Setup success Response, received UPFunctionFeatures= %v ", rsp.UPFunctionFeatures)
			upf.UPFunctionFeatures = rsp.UPFunctionFeatures
		}

		if rsp.UserPlaneIPResourceInformation != nil {
			upf.UPIPInfo = *rsp.UserPlaneIPResourceInformation

			if upf.UPIPInfo.Assosi && upf.UPIPInfo.Assoni && upf.UPIPInfo.SourceInterface == pfcpType.SourceInterfaceAccess &&
				upf.UPIPInfo.V4 && !upf.UPIPInfo.Ipv4Address.Equal(net.IPv4zero) {
				logger.PfcpLog.Infof("UPF[%s] received N3 interface IP[%v], network instance[%v] and TEID[%v]",
					upf.NodeID.ResolveNodeIdToIp().String(), upf.UPIPInfo.Ipv4Address,
					string(upf.UPIPInfo.NetworkInstance), upf.UPIPInfo.TeidRange)

				// reset the N3 interface of UPF
				upf.N3Interfaces = make([]smf_context.UPFInterfaceInfo, 0)

				// Insert N3 interface info from UPF
				n3Interface := smf_context.UPFInterfaceInfo{}
				n3Interface.NetworkInstance = string(upf.UPIPInfo.NetworkInstance)
				n3Interface.IPv4EndPointAddresses = append(n3Interface.IPv4EndPointAddresses, upf.UPIPInfo.Ipv4Address)
				upf.N3Interfaces = append(upf.N3Interfaces, n3Interface)
			}

			logger.PfcpLog.Infof("UPF(%s)[%s] setup association success",
				upf.NodeID.ResolveNodeIdToIp().String(), upf.UPIPInfo.NetworkInstance)
		} else {
			logger.PfcpLog.Errorln("pfcp association setup response has no UserPlane IP Resource Information")
		}
	}
}

func HandlePfcpAssociationReleaseRequest(msg *pfcpUdp.Message) {
	pfcpMsg := msg.PfcpMessage.Body.(pfcp.PFCPAssociationReleaseRequest)

	var cause pfcpType.Cause
	upf := smf_context.RetrieveUPFNodeByNodeID(*pfcpMsg.NodeID)

	if upf != nil {
		smf_context.RemoveUPFNodeByNodeID(*pfcpMsg.NodeID)
		cause.CauseValue = pfcpType.CauseRequestAccepted
	} else {
		cause.CauseValue = pfcpType.CauseNoEstablishedPfcpAssociation
	}

	pfcp_message.SendPfcpAssociationReleaseResponse(*pfcpMsg.NodeID, cause, upf.Port)
}

func HandlePfcpAssociationReleaseResponse(msg *pfcpUdp.Message) {
	pfcpMsg := msg.PfcpMessage.Body.(pfcp.PFCPAssociationReleaseResponse)

	if pfcpMsg.Cause.CauseValue == pfcpType.CauseRequestAccepted {
		smf_context.RemoveUPFNodeByNodeID(*pfcpMsg.NodeID)
	}
}

func HandlePfcpSessionEstablishmentResponse(msg *pfcpUdp.Message) {
	rsp := msg.PfcpMessage.Body.(pfcp.PFCPSessionEstablishmentResponse)
	logger.PfcpLog.Infoln("In HandlePfcpSessionEstablishmentResponse")

	SEID := msg.PfcpMessage.Header.SEID
	if SEID == 0 {
		if eventData, ok := msg.EventData.(pfcpUdp.PfcpEventData); !ok {
			logger.PfcpLog.Warnf("PFCP Session Establish Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := smf_context.GetSMContextBySEID(SEID)
	smContext.SMLock.Lock()

	// Get NodeId from Seq:NodeId Map
	seq := msg.PfcpMessage.Header.SequenceNumber
	nodeID := pfcp_message.FetchPfcpTxn(seq)

	if rsp.UPFSEID != nil {
		// NodeIDtoIP := rsp.NodeID.ResolveNodeIdToIp().String()
		NodeIDtoIP := nodeID.ResolveNodeIdToIp().String()
		pfcpSessionCtx := smContext.PFCPContext[NodeIDtoIP]
		pfcpSessionCtx.RemoteSEID = rsp.UPFSEID.Seid
	}
	smContext.SubPfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse rsp.UPFSEID.Seid [%v] ", rsp.UPFSEID.Seid)

	// UE IP-Addr(only v4 supported)
	if rsp.CreatedPDR != nil && rsp.CreatedPDR.UEIPAddress != nil {
		smContext.SubPfcpLog.Infof("upf provided ue ip address [%v]", rsp.CreatedPDR.UEIPAddress.Ipv4Address)

		// Release previous locally allocated UE IP-Addr
		smContext.ReleaseUeIpAddr()

		// Update with one received from UPF
		smContext.PDUAddress.Ip = rsp.CreatedPDR.UEIPAddress.Ipv4Address
		smContext.PDUAddress.UpfProvided = true
	}
	smContext.SMLock.Unlock()

	// Get N3 interface UPF
	ANUPF := smContext.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode

	if ANUPF.UPF.NodeID.ResolveNodeIdToIp().Equal(nodeID.ResolveNodeIdToIp()) {
		// UPF Accept
		if rsp.Cause.CauseValue == pfcpType.CauseRequestAccepted {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionEstablishSuccess
			smContext.SubPfcpLog.Infof("PFCP Session Establishment accepted")
		} else {
			smContext.SBIPFCPCommunicationChan <- smf_context.SessionEstablishFailed
			smContext.SubPfcpLog.Errorf("PFCP Session Establishment rejected with cause [%v]", rsp.Cause.CauseValue)
			if rsp.Cause.CauseValue ==
				pfcpType.CauseNoEstablishedPfcpAssociation {
				SetUpfInactive(*rsp.NodeID, msg.PfcpMessage.Header.MessageType)
			}
		}
	}

	if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == smf_context.AddingPSA {
			smContext.SubPfcpLog.Infoln("Keep Adding PSAndULCL")
			producer.AddPDUSessionAnchorAndULCL(smContext, *rsp.NodeID)
			smContext.BPManager.BPStatus = smf_context.AddingPSA
		}
	}
}

func HandlePfcpSessionModificationResponse(msg *pfcpUdp.Message) {
	pfcpRsp := msg.PfcpMessage.Body.(pfcp.PFCPSessionModificationResponse)

	SEID := msg.PfcpMessage.Header.SEID

	if SEID == 0 {
		if eventData, ok := msg.EventData.(pfcpUdp.PfcpEventData); !ok {
			logger.PfcpLog.Warnf("PFCP Session Modification Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := smf_context.GetSMContextBySEID(SEID)

	logger.PfcpLog.Infoln("In HandlePfcpSessionModificationResponse")

	if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == smf_context.AddingPSA {
			smContext.SubPfcpLog.Infoln("Keep Adding PSAAndULCL")

			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			producer.AddPDUSessionAnchorAndULCL(smContext, upfNodeID)
		}
	}

	if pfcpRsp.Cause.CauseValue == pfcpType.CauseRequestAccepted {
		smContext.SubPduSessLog.Infoln("PFCP Modification Response Accept")
		if smContext.SMContextState == smf_context.SmStatePfcpModify {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Tracef("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() {
				smContext.SBIPFCPCommunicationChan <- smf_context.SessionUpdateSuccess
			}

			if smf_context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
				if smContext.BPManager.BPStatus == smf_context.UnInitialized {
					smContext.SubPfcpLog.Infoln("Add PSAAndULCL")
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

	smContext.SubCtxLog.Traceln("PFCP Session Context")
	for _, ctx := range smContext.PFCPContext {
		smContext.SubCtxLog.Traceln(ctx.String())
	}
}

func HandlePfcpSessionDeletionResponse(msg *pfcpUdp.Message) {
	logger.PfcpLog.Infof("Handle PFCP Session Deletion Response")
	pfcpRsp := msg.PfcpMessage.Body.(pfcp.PFCPSessionDeletionResponse)
	SEID := msg.PfcpMessage.Header.SEID

	if SEID == 0 {
		if eventData, ok := msg.EventData.(pfcpUdp.PfcpEventData); !ok {
			logger.PfcpLog.Warnf("PFCP Session Deletion Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := smf_context.GetSMContextBySEID(SEID)

	if smContext == nil {
		logger.PfcpLog.Warnf("PFCP Session Deletion Response found SM context nil, response discarded")
		return
		// TODO fix: SEID should be the value sent by UPF but now the SEID value is from sm context
	}

	if pfcpRsp.Cause.CauseValue == pfcpType.CauseRequestAccepted {
		if smContext.SMContextState == smf_context.SmStatePfcpRelease {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Tracef("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

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

func HandlePfcpSessionReportRequest(msg *pfcpUdp.Message) {
	req := msg.PfcpMessage.Body.(pfcp.PFCPSessionReportRequest)

	SEID := msg.PfcpMessage.Header.SEID
	smContext := smf_context.GetSMContextBySEID(SEID)
	seqFromUPF := msg.PfcpMessage.Header.SequenceNumber

	var cause pfcpType.Cause
	var pfcpSRflag pfcpType.PFCPSRRspFlags

	if smContext == nil {
		logger.PfcpLog.Warnf("PFCP Session Report Request Found SM Context NULL, Request Rejected")
		cause.CauseValue = pfcpType.CauseRequestRejected

		// Rejecting buffering at UPF since not able to process Session Report Request
		pfcpSRflag.Drobu = true
		// TODO fix: SEID should be the value sent by UPF but now the SEID value is from sm context
		pfcp_message.SendPfcpSessionReportResponse(msg.RemoteAddr, cause, pfcpSRflag, seqFromUPF, SEID)
		return
	}

	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	if smContext.UpCnxState == models.UpCnxState_DEACTIVATED {
		if req.ReportType.Dldr {
			downlinkDataReport := req.DownlinkDataReport

			if downlinkDataReport.DownlinkDataServiceInformation != nil {
				smContext.SubPfcpLog.Warnf("PFCP Session Report Request DownlinkDataServiceInformation handling is not implemented")
			}

			n1n2Request := models.N1N2MessageTransferRequest{}

			// TS 23.502 4.2.3.3 3a. Send Namf_Communication_N1N2MessageTransfer Request, SMF->AMF
			if n2SmBuf, err := smf_context.BuildPDUSessionResourceSetupRequestTransfer(smContext); err != nil {
				smContext.SubPduSessLog.Errorln("Build PDUSessionResourceSetupRequestTransfer failed:", err)
			} else {
				n1n2Request.BinaryDataN2Information = n2SmBuf
			}

			// n1n2FailureTxfNotifURI to be added in n1n2 request transfer.
			// It is used as path by AMF to send failure notification message towards SMF
			n1n2FailureTxfNotifURI := "/nsmf-callback/sm-n1n2failnotify/"
			n1n2FailureTxfNotifURI += smContext.Ref

			n1n2Request.JsonData = &models.N1N2MessageTransferReqData{
				PduSessionId: smContext.PDUSessionID,
				SkipInd:      false,
				// Temporarily assign SMF itself, TODO: TS 23.502 4.2.3.3 5. Namf_Communication_N1N2TransferFailureNotification
				N1n2FailureTxfNotifURI: fmt.Sprintf("%s://%s:%d%s",
					smf_context.SMF_Self().URIScheme,
					smf_context.SMF_Self().RegisterIPv4,
					smf_context.SMF_Self().SBIPort,
					n1n2FailureTxfNotifURI),
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

			rspData, _, err := smContext.CommunicationClient.
				N1N2MessageCollectionDocumentApi.
				N1N2MessageTransfer(context.Background(), smContext.Supi, n1n2Request)
			if err != nil {
				smContext.SubPfcpLog.Warnf("Send N1N2Transfer failed")
			}
			if rspData.Cause == models.N1N2MessageTransferCause_ATTEMPTING_TO_REACH_UE {
				smContext.SubPfcpLog.Infof("Receive %v, AMF is able to page the UE", rspData.Cause)

				pfcpSRflag.Drobu = false
				cause.CauseValue = pfcpType.CauseRequestAccepted
			}
			if rspData.Cause == models.N1N2MessageTransferCause_UE_NOT_RESPONDING {
				smContext.SubPfcpLog.Infof("Receive %v, UE is not responding to N1N2 transfer message", rspData.Cause)
				// TODO: TS 23.502 4.2.3.3 3c. Failure indication

				// Adding Session report flag to drop buffered packet at UPF
				pfcpSRflag.Drobu = true

				// Adding Cause rejected since N1N2 Transfer message got rejected.
				cause.CauseValue = pfcpType.CauseRequestRejected
			}

			// Sending Session Report Response to UPF.
			smContext.SubPfcpLog.Infof("Sending Session Report to UPF with Cause %v", cause.CauseValue)
			pfcp_message.SendPfcpSessionReportResponse(msg.RemoteAddr, cause, pfcpSRflag, seqFromUPF, SEID)
		}
	}

	// TS 23.502 4.2.3.3 2b. Send Data Notification Ack, SMF->UPF
	//	cause.CauseValue = pfcpType.CauseRequestAccepted
	// TODO fix: SEID should be the value sent by UPF but now the SEID value is from sm context
	// pfcp_message.SendPfcpSessionReportResponse(msg.RemoteAddr, cause, seqFromUPF, SEID)
}
