// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package adapter

import (
	"net"
	"sync"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/pfcp/ies"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
)

func init() {
	PfcpTxns = make(map[uint32]*context.NodeID)
}

var (
	PfcpTxns    map[uint32]*context.NodeID
	PfcpTxnLock sync.Mutex
)

func FetchPfcpTxn(seqNo uint32) (upNodeID *context.NodeID) {
	PfcpTxnLock.Lock()
	defer PfcpTxnLock.Unlock()
	if upNodeID = PfcpTxns[seqNo]; upNodeID != nil {
		delete(PfcpTxns, seqNo)
	}
	return upNodeID
}

func InsertPfcpTxn(seqNo uint32, upNodeID *context.NodeID) {
	PfcpTxnLock.Lock()
	defer PfcpTxnLock.Unlock()
	PfcpTxns[seqNo] = upNodeID
}

/*
This function is called when smf runs with upfadapter and the communication between

	them is sync. smf already holds the lock before calling to the below API, so not required
	upfLock in handler functions
*/
func HandleAdapterPfcpRsp(pfcpMsg message.Message, evtData *udp.PfcpEventData) error {
	switch pfcpMsg.MessageType() {
	case message.MsgTypeAssociationSetupResponse:
		msg := udp.Message{PfcpMessage: pfcpMsg}
		HandlePfcpAssociationSetupResponse(&msg)
	case message.MsgTypeHeartbeatResponse:
		msg := udp.Message{PfcpMessage: pfcpMsg}
		HandlePfcpHeartbeatResponse(&msg)
	case message.MsgTypeSessionEstablishmentResponse:
		msg := udp.Message{PfcpMessage: pfcpMsg, EventData: *evtData}
		HandlePfcpSessionEstablishmentResponse(&msg)
	case message.MsgTypeSessionModificationResponse:
		msg := udp.Message{PfcpMessage: pfcpMsg, EventData: *evtData}
		HandlePfcpSessionModificationResponse(&msg)
	case message.MsgTypeSessionDeletionResponse:
		msg := udp.Message{PfcpMessage: pfcpMsg, EventData: *evtData}
		HandlePfcpSessionDeletionResponse(&msg)
	default:
		logger.PfcpLog.Errorf("upf adapter invalid msg type: %v", pfcpMsg)
	}
	return nil
}

func HandlePfcpAssociationSetupResponse(msg *udp.Message) {
	rsp, ok := msg.PfcpMessage.(*message.AssociationSetupResponse)
	if !ok {
		logger.PfcpLog.Errorln("invalid PFCP Association Setup Response")
		return
	}

	nodeIDIE := rsp.NodeID
	if nodeIDIE == nil {
		logger.PfcpLog.Errorln("pfcp association setup response has no NodeID")
		return
	}

	nodeIDStr, err := nodeIDIE.NodeID()
	if err != nil {
		logger.PfcpLog.Errorf("pfcp association setup response NodeID error: %v", err)
		return
	}

	nodeID := context.NewNodeID(nodeIDStr)

	if rsp.Cause == nil {
		logger.PfcpLog.Errorln("pfcp association setup response has no cause")
		return
	}

	causeValue, err := rsp.Cause.Cause()
	if err != nil {
		logger.PfcpLog.Errorf("pfcp association setup response cause error: %v", err)
		return
	}

	var userPlaneIPResourceInformation *context.UserPlaneIPResourceInformation
	if len(rsp.UserPlaneIPResourceInformation) != 0 {
		userPlaneIPResourceInformation, err = ies.UnmarshalUEIPInformationBinary(rsp.UserPlaneIPResourceInformation[0].Payload)
		if err != nil {
			logger.PfcpLog.Errorf("failed to get UserPlaneIPResourceInformation: %+v", err)
			return
		}
	}

	if causeValue == ie.CauseRequestAccepted {
		logger.PfcpLog.Infof("handle PFCP Association Setup Response with NodeID[%s]", nodeID.ResolveNodeIdToIp().String())

		upf := context.RetrieveUPFNodeByNodeID(*nodeID)
		if upf == nil {
			logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
			return
		}

		// validate if DNNs served by UPF matches with the one provided by UPF
		if userPlaneIPResourceInformation != nil {
			upfProvidedDnn := string(userPlaneIPResourceInformation.NetworkInstance)
			if !upf.IsDnnConfigured(upfProvidedDnn) {
				logger.PfcpLog.Errorf("handle PFCP Association Setup Response, DNN mismatch, [%v] is not configured ", upfProvidedDnn)
				return
			}
		}

		upf.UPFStatus = context.AssociatedSetUpSuccess
		if rsp.RecoveryTimeStamp == nil {
			logger.PfcpLog.Errorln("pfcp association setup response has no RecoveryTimeStamp")
			return
		}
		recoveryTimestamp, err := rsp.RecoveryTimeStamp.RecoveryTimeStamp()
		if err != nil {
			logger.PfcpLog.Errorf("pfcp association setup response RecoveryTimeStamp error: %v", err)
			return
		}
		upf.RecoveryTimeStamp = context.RecoveryTimeStamp{
			RecoveryTimeStamp: recoveryTimestamp,
		}
		upf.NHeartBeat = 0 // reset Heartbeat attempt to 0

		if rsp.UserPlaneIPResourceInformation != nil {
			upf.UPIPInfo = *userPlaneIPResourceInformation

			if upf.UPIPInfo.Assosi && upf.UPIPInfo.Assoni && upf.UPIPInfo.SourceInterface == ie.SrcInterfaceAccess &&
				upf.UPIPInfo.V4 && !upf.UPIPInfo.Ipv4Address.Equal(net.IPv4zero) {
				logger.PfcpLog.Infof("UPF[%s] received N3 interface IP[%v], network instance[%v] and TEID[%v]",
					upf.NodeID.ResolveNodeIdToIp().String(), upf.UPIPInfo.Ipv4Address,
					string(upf.UPIPInfo.NetworkInstance), upf.UPIPInfo.TeidRange)

				// reset the N3 interface of UPF
				upf.N3Interfaces = make([]context.UPFInterfaceInfo, 0)

				// Insert N3 interface info from UPF
				n3Interface := context.UPFInterfaceInfo{}
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

func HandlePfcpHeartbeatResponse(msg *udp.Message) {
	rsp, ok := msg.PfcpMessage.(*message.HeartbeatResponse)
	if !ok {
		logger.PfcpLog.Errorln("invalid PFCP Heartbeat Response")
		return
	}

	// Get NodeId from Seq:NodeId Map
	seq := rsp.Sequence()
	nodeID := FetchPfcpTxn(seq)

	if nodeID == nil {
		logger.PfcpLog.Errorf("no pending pfcp heartbeat response for sequence no: %v", seq)
		// metrics.IncrementN4MsgStats(context.SMF_Self().NfInstanceID, pfcpmsgtypes.PfcpMsgTypeString(msg.PfcpMessage.Header.MessageType), "In", "Failure", "invalid_seqno")
		return
	}

	logger.PfcpLog.Debugf("handle pfcp heartbeat response seq[%d] with NodeID[%v, %s]", seq, nodeID, nodeID.ResolveNodeIdToIp().String())

	upf := context.RetrieveUPFNodeByNodeID(*nodeID)
	if upf == nil {
		logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
		// metrics.IncrementN4MsgStats(context.SMF_Self().NfInstanceID, pfcpmsgtypes.PfcpMsgTypeString(msg.PfcpMessage.Header.MessageType), "In", "Failure", "unknown_upf")
		return
	}

	if rsp.RecoveryTimeStamp == nil {
		logger.PfcpLog.Errorln("pfcp heartbeat response has no RecoveryTimeStamp")
		return
	}

	recoveryTimestamp, err := rsp.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		logger.PfcpLog.Errorf("pfcp heartbeat response RecoveryTimeStamp error: %v", err)
		return
	}

	if recoveryTimestamp != upf.RecoveryTimeStamp.RecoveryTimeStamp {
		// change UPF state to not associated so that
		// PFCP Association can be initiated again
		upf.UPFStatus = context.NotAssociated
		logger.PfcpLog.Warnf("PFCP Heartbeat Response, upf [%v] recovery timestamp changed", upf.NodeID)

		// TODO: Session cleanup required and updated to AMF/PCF
		// metrics.IncrementN4MsgStats(context.SMF_Self().NfInstanceID, pfcpmsgtypes.PfcpMsgTypeString(msg.PfcpMessage.Header.MessageType), "In", "Failure", "RecoveryTimeStamp_mismatch")
	}

	upf.NHeartBeat = 0 // reset Heartbeat attempt to 0
}

func HandlePfcpSessionEstablishmentResponse(msg *udp.Message) {
	rsp, ok := msg.PfcpMessage.(*message.SessionEstablishmentResponse)
	if !ok {
		logger.PfcpLog.Errorln("invalid PFCP Session Establishment Response")
		return
	}
	logger.PfcpLog.Infoln("in HandlePfcpSessionEstablishmentResponse")

	SEID := rsp.SEID()
	if SEID == 0 {
		if eventData, ok := msg.EventData.(udp.PfcpEventData); !ok {
			logger.PfcpLog.Warnf("PFCP Session Establish Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := context.GetSMContextBySEID(SEID)
	logger.PfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse SEID %v", SEID)
	logger.PfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse smContext %+v", smContext)

	// Get NodeId from Seq:NodeId Map
	seq := rsp.Sequence()
	nodeID := FetchPfcpTxn(seq)

	if rsp.UPFSEID != nil {
		NodeIDtoIP := nodeID.ResolveNodeIdToIp().String()
		pfcpSessionCtx := smContext.PFCPContext[NodeIDtoIP]
		rspUPFseid, err := rsp.UPFSEID.FSEID()
		if err != nil {
			logger.PfcpLog.Errorf("pfcp session establishment response UPFSEID error: %v", err)
			return
		}
		pfcpSessionCtx.RemoteSEID = rspUPFseid.SEID
		smContext.SubPfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse rsp.UPFSEID.Seid [%v] ", rspUPFseid.SEID)
	}

	// Get N3 interface UPF
	ANUPF := smContext.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode

	if ANUPF.UPF.NodeID.ResolveNodeIdToIp().Equal(nodeID.ResolveNodeIdToIp()) {
		if rsp.Cause == nil {
			logger.PfcpLog.Errorln("pfcp session establishment response has no cause")
			return
		}
		causeValue, err := rsp.Cause.Cause()
		if err != nil {
			logger.PfcpLog.Errorf("pfcp session establishment response cause error: %v", err)
			return
		}
		// UPF Accept
		if causeValue == ie.CauseRequestAccepted {
			smContext.SBIPFCPCommunicationChan <- context.SessionEstablishSuccess
			smContext.SubPfcpLog.Infof("PFCP Session Establishment accepted")
		} else {
			smContext.SBIPFCPCommunicationChan <- context.SessionEstablishFailed
			smContext.SubPfcpLog.Errorf("PFCP Session Establishment rejected with cause [%v]", causeValue)
			if causeValue == ie.CauseNoEstablishedPFCPAssociation {
				rspNodeIDStr, err := rsp.NodeID.NodeID()
				if err != nil {
					logger.PfcpLog.Errorf("pfcp session establishment response NodeID error: %v", err)
					return
				}
				rspNodeID := context.NewNodeID(rspNodeIDStr)
				SetUpfInactive(*rspNodeID)
			}
		}
	}
}

func HandlePfcpSessionModificationResponse(msg *udp.Message) {
	pfcpRsp, ok := msg.PfcpMessage.(*message.SessionModificationResponse)
	if !ok {
		logger.PfcpLog.Errorln("invalid PFCP Session Modification Response")
		return
	}
	logger.PfcpLog.Infoln("in HandlePfcpSessionModificationResponse")

	cause := pfcpRsp.Cause
	if cause == nil {
		logger.PfcpLog.Warnf("PFCP Session Modification Response found invalid cause, response discarded")
		return
	}
	causeValue, err := cause.Cause()
	if err != nil {
		logger.PfcpLog.Errorf("PFCP Session Modification Response cause error: %v", err)
		return
	}

	logger.PfcpLog.Infof("in HandlePfcpSessionModificationResponse pfcpRsp.Cause.CauseValue = [%v], accepted?? %v", causeValue, causeValue == ie.CauseRequestAccepted)

	SEID := pfcpRsp.SEID()
	logger.PfcpLog.Infof("in HandlePfcpSessionModificationResponse SEID %v", SEID)

	smContext := context.GetSMContextBySEID(SEID)
	logger.PfcpLog.Infof("in HandlePfcpSessionModificationResponse smContext found by SEID %v", smContext)

	if SEID == 0 {
		if eventData, ok := msg.EventData.(udp.PfcpEventData); !ok {
			logger.PfcpLog.Warnf("PFCP Session Modification Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}

	if causeValue == ie.CauseRequestAccepted {
		smContext.SubPduSessLog.Infoln("PFCP Modification Response Accept")
		if smContext.SMContextState == context.SmStatePfcpModify {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Tracef("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() {
				smContext.SBIPFCPCommunicationChan <- context.SessionUpdateSuccess
			}
		}

		smContext.SubPfcpLog.Infof("PFCP Session Modification Success[%d]\n", SEID)
	} else {
		smContext.SubPfcpLog.Infof("PFCP Session Modification Failed[%d]\n", SEID)
		if smContext.SMContextState == context.SmStatePfcpModify {
			smContext.SBIPFCPCommunicationChan <- context.SessionUpdateFailed
		}
	}

	smContext.SubCtxLog.Traceln("PFCP Session Context")
	for _, ctx := range smContext.PFCPContext {
		smContext.SubCtxLog.Traceln(ctx.String())
	}
}

func HandlePfcpSessionDeletionResponse(msg *udp.Message) {
	pfcpRsp, ok := msg.PfcpMessage.(*message.SessionDeletionResponse)
	if !ok {
		logger.PfcpLog.Errorln("invalid PFCP Session Deletion Response")
		return
	}
	logger.PfcpLog.Infof("handle PFCP Session Deletion Response")
	SEID := pfcpRsp.SEID()

	if SEID == 0 {
		if eventData, ok := msg.EventData.(udp.PfcpEventData); !ok {
			logger.PfcpLog.Warnf("PFCP Session Deletion Response found invalid event data, response discarded")
			return
		} else {
			SEID = eventData.LSEID
		}
	}
	smContext := context.GetSMContextBySEID(SEID)

	if smContext == nil {
		logger.PfcpLog.Warnf("PFCP Session Deletion Response found SM context nil, response discarded")
		return
	}

	cause := pfcpRsp.Cause
	if cause == nil {
		logger.PfcpLog.Warnf("PFCP Session Deletion Response found invalid cause, response discarded")
		return
	}

	causeValue, err := cause.Cause()
	if err != nil {
		logger.PfcpLog.Errorf("PFCP Session Deletion Response cause error: %v", err)
		return
	}

	if causeValue == ie.CauseRequestAccepted {
		if smContext.SMContextState == context.SmStatePfcpRelease {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Tracef("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() && !smContext.LocalPurged {
				smContext.SBIPFCPCommunicationChan <- context.SessionReleaseSuccess
			}
		}
		smContext.SubPfcpLog.Infof("PFCP Session Deletion Success[%d]\n", SEID)
	} else {
		if smContext.SMContextState == context.SmStatePfcpRelease && !smContext.LocalPurged {
			smContext.SBIPFCPCommunicationChan <- context.SessionReleaseSuccess
		}
		smContext.SubPfcpLog.Infof("PFCP Session Deletion Failed[%d]\n", SEID)
	}
}

func SetUpfInactive(nodeID context.NodeID) {
	upf := context.RetrieveUPFNodeByNodeID(nodeID)
	if upf == nil {
		logger.PfcpLog.Errorf("can't find UPF[%s]", nodeID.ResolveNodeIdToIp().String())
		// metrics.IncrementN4MsgStats(context.SMF_Self().NfInstanceID,
		//	pfcpmsgtypes.PfcpMsgTypeString(msgType),
		//	"In", "Failure", "unknown_upf")
		return
	}

	upf.UPFStatus = context.NotAssociated
	upf.NHeartBeat = 0 // reset Heartbeat attempt to 0
}
