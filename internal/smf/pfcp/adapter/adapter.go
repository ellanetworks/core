package adapter

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpType"
	"github.com/omec-project/pfcp/pfcpUdp"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
)

func init() {
	PfcpTxns = make(map[uint32]*pfcpType.NodeID)
}

var (
	PfcpTxns    map[uint32]*pfcpType.NodeID
	PfcpTxnLock sync.Mutex
)

func FetchPfcpTxn(seqNo uint32) (upNodeID *pfcpType.NodeID) {
	PfcpTxnLock.Lock()
	defer PfcpTxnLock.Unlock()
	if upNodeID = PfcpTxns[seqNo]; upNodeID != nil {
		delete(PfcpTxns, seqNo)
	}
	return upNodeID
}

func InsertPfcpTxn(seqNo uint32, upNodeID *pfcpType.NodeID) {
	PfcpTxnLock.Lock()
	defer PfcpTxnLock.Unlock()
	PfcpTxns[seqNo] = upNodeID
}

/*
This function is called when smf runs with upfadapter and the communication between

	them is sync. smf already holds the lock before calling to the below API, so not required
	upfLock in handler functions
*/
func HandleAdapterPfcpRsp(pfcpMsg pfcp.Message, evtData *pfcpUdp.PfcpEventData) error {
	pfcpBodyJson, _ := json.Marshal(pfcpMsg.Body)

	switch pfcpMsg.Header.MessageType {
	case pfcp.PFCP_ASSOCIATION_SETUP_RESPONSE:
		pfcpRsp := pfcp.PFCPAssociationSetupResponse{}
		json.Unmarshal(pfcpBodyJson, &pfcpRsp)

		pfcpMsg.Body = pfcpRsp
		msg := pfcpUdp.Message{PfcpMessage: &pfcpMsg}
		HandlePfcpAssociationSetupResponse(&msg)

	case pfcp.PFCP_HEARTBEAT_RESPONSE:
		pfcpRsp := pfcp.HeartbeatResponse{}
		json.Unmarshal(pfcpBodyJson, &pfcpRsp)

		pfcpMsg.Body = pfcpRsp
		msg := pfcpUdp.Message{PfcpMessage: &pfcpMsg}
		HandlePfcpHeartbeatResponse(&msg)
	case pfcp.PFCP_SESSION_ESTABLISHMENT_RESPONSE:
		pfcpRsp := pfcp.PFCPSessionEstablishmentResponse{}
		json.Unmarshal(pfcpBodyJson, &pfcpRsp)

		pfcpMsg.Body = pfcpRsp
		msg := pfcpUdp.Message{PfcpMessage: &pfcpMsg, EventData: *evtData}
		HandlePfcpSessionEstablishmentResponse(&msg)
	case pfcp.PFCP_SESSION_MODIFICATION_RESPONSE:
		pfcpRsp := pfcp.PFCPSessionModificationResponse{}
		json.Unmarshal(pfcpBodyJson, &pfcpRsp)

		pfcpMsg.Body = pfcpRsp
		msg := pfcpUdp.Message{PfcpMessage: &pfcpMsg, EventData: *evtData}
		HandlePfcpSessionModificationResponse(&msg)
	case pfcp.PFCP_SESSION_DELETION_RESPONSE:
		pfcpRsp := pfcp.PFCPSessionDeletionResponse{}
		json.Unmarshal(pfcpBodyJson, &pfcpRsp)

		pfcpMsg.Body = pfcpRsp
		msg := pfcpUdp.Message{PfcpMessage: &pfcpMsg, EventData: *evtData}
		HandlePfcpSessionDeletionResponse(&msg)
	default:
		logger.PfcpLog.Errorf("upf adapter invalid msg type: %v", pfcpMsg)
	}

	return nil
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

		upf := smf_context.RetrieveUPFNodeByNodeID(*nodeID)
		if upf == nil {
			logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
			return
		}

		// validate if DNNs served by UPF matches with the one provided by UPF
		if rsp.UserPlaneIPResourceInformation != nil {
			upfProvidedDnn := string(rsp.UserPlaneIPResourceInformation.NetworkInstance)
			if !upf.IsDnnConfigured(upfProvidedDnn) {
				logger.PfcpLog.Errorf("Handle PFCP Association Setup Response, DNN mismatch, [%v] is not configured ", upfProvidedDnn)
				return
			}
		}

		upf.UPFStatus = smf_context.AssociatedSetUpSuccess
		upf.RecoveryTimeStamp = *rsp.RecoveryTimeStamp
		upf.NHeartBeat = 0 // reset Heartbeat attempt to 0

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

			logger.PfcpLog.Infof("UPF(%s)[%s] setup association",
				upf.NodeID.ResolveNodeIdToIp().String(), upf.UPIPInfo.NetworkInstance)
		} else {
			logger.PfcpLog.Errorln("pfcp association setup response has no UserPlane IP Resource Information")
		}
	}
}

func HandlePfcpHeartbeatResponse(msg *pfcpUdp.Message) {
	rsp := msg.PfcpMessage.Body.(pfcp.HeartbeatResponse)

	// Get NodeId from Seq:NodeId Map
	seq := msg.PfcpMessage.Header.SequenceNumber
	nodeID := FetchPfcpTxn(seq)

	if nodeID == nil {
		logger.PfcpLog.Errorf("No pending pfcp heartbeat response for sequence no: %v", seq)
		// metrics.IncrementN4MsgStats(smf_context.SMF_Self().NfInstanceID, pfcpmsgtypes.PfcpMsgTypeString(msg.PfcpMessage.Header.MessageType), "In", "Failure", "invalid_seqno")
		return
	}

	logger.PfcpLog.Debugf("handle pfcp heartbeat response seq[%d] with NodeID[%v, %s]", seq, nodeID, nodeID.ResolveNodeIdToIp().String())

	upf := smf_context.RetrieveUPFNodeByNodeID(*nodeID)
	if upf == nil {
		logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
		// metrics.IncrementN4MsgStats(smf_context.SMF_Self().NfInstanceID, pfcpmsgtypes.PfcpMsgTypeString(msg.PfcpMessage.Header.MessageType), "In", "Failure", "unknown_upf")
		return
	}

	if *rsp.RecoveryTimeStamp != upf.RecoveryTimeStamp {
		// change UPF state to not associated so that
		// PFCP Association can be initiated again
		upf.UPFStatus = smf_context.NotAssociated
		logger.PfcpLog.Warnf("PFCP Heartbeat Response, upf [%v] recovery timestamp changed", upf.NodeID)

		// TODO: Session cleanup required and updated to AMF/PCF
		// metrics.IncrementN4MsgStats(smf_context.SMF_Self().NfInstanceID, pfcpmsgtypes.PfcpMsgTypeString(msg.PfcpMessage.Header.MessageType), "In", "Failure", "RecoveryTimeStamp_mismatch")
	}

	upf.NHeartBeat = 0 // reset Heartbeat attempt to 0
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
	logger.PfcpLog.Infof("In HandlePfcpSessionEstablishmentResponse SEID %v", SEID)
	logger.PfcpLog.Infof("In HandlePfcpSessionEstablishmentResponse smContext %+v", smContext)

	// Get NodeId from Seq:NodeId Map
	seq := msg.PfcpMessage.Header.SequenceNumber
	nodeID := FetchPfcpTxn(seq)

	if rsp.UPFSEID != nil {
		NodeIDtoIP := nodeID.ResolveNodeIdToIp().String()
		pfcpSessionCtx := smContext.PFCPContext[NodeIDtoIP]
		pfcpSessionCtx.RemoteSEID = rsp.UPFSEID.Seid
		smContext.SubPfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse rsp.UPFSEID.Seid [%v] ", rsp.UPFSEID.Seid)
	}

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
}

func HandlePfcpSessionModificationResponse(msg *pfcpUdp.Message) {
	logger.PfcpLog.Infoln("In HandlePfcpSessionModificationResponse")

	pfcpRsp := msg.PfcpMessage.Body.(pfcp.PFCPSessionModificationResponse)
	logger.PfcpLog.Infof("In HandlePfcpSessionModificationResponse pfcpRsp.Cause.CauseValue = [%v], accepted?? %v", pfcpRsp.Cause.CauseValue, pfcpRsp.Cause.CauseValue == pfcpType.CauseRequestAccepted)

	SEID := msg.PfcpMessage.Header.SEID
	logger.PfcpLog.Infof("In HandlePfcpSessionModificationResponse SEID %v", SEID)

	smContext := smf_context.GetSMContextBySEID(SEID)
	logger.PfcpLog.Infof("In HandlePfcpSessionModificationResponse smContext found by SEID %v", smContext)

	if SEID == 0 {
		if eventData, ok := msg.EventData.(pfcpUdp.PfcpEventData); !ok {
			logger.PfcpLog.Warnf("PFCP Session Modification Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	// smContext := smf_context.GetSMContextBySEID(SEID)

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

func SetUpfInactive(nodeID pfcpType.NodeID, msgType pfcp.MessageType) {
	upf := smf_context.RetrieveUPFNodeByNodeID(nodeID)
	if upf == nil {
		logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
		// metrics.IncrementN4MsgStats(smf_context.SMF_Self().NfInstanceID,
		//	pfcpmsgtypes.PfcpMsgTypeString(msgType),
		//	"In", "Failure", "unknown_upf")
		return
	}

	upf.UPFStatus = smf_context.NotAssociated
	upf.NHeartBeat = 0 // reset Heartbeat attempt to 0
}
