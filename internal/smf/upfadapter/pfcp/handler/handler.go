package handler

import (
	"encoding/json"
	"fmt"

	"upf-adapter/config"
	"upf-adapter/logger"

	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpType"
	"github.com/omec-project/pfcp/pfcpUdp"
)

func HandlePfcpSendError(msg *pfcp.Message, pfcpErr error) {
	logger.PfcpLog.Errorf("send of PFCP msg [%v] failed with error [%v] ",
		msg.Header.MessageType, pfcpErr.Error())

	switch msg.Header.MessageType {
	case pfcp.PFCP_ASSOCIATION_SETUP_REQUEST:
		handleSendPfcpAssoSetReqError(msg, pfcpErr)
	case pfcp.PFCP_HEARTBEAT_REQUEST:
		handleSendPfcpHeartbeatReqError(msg, pfcpErr)
	case pfcp.PFCP_SESSION_ESTABLISHMENT_REQUEST:
		handleSendPfcpSessEstReqError(msg, pfcpErr)
	case pfcp.PFCP_SESSION_MODIFICATION_REQUEST:
		handleSendPfcpSessModReqError(msg, pfcpErr)
	case pfcp.PFCP_SESSION_DELETION_REQUEST:
		handleSendPfcpSessRelReqError(msg, pfcpErr)
	default:
		logger.PfcpLog.Errorf("Unable to send PFCP packet type [%v] and content [%v]",
			msg.Header.MessageType, msg)
	}
}

func handleSendPfcpAssoSetReqError(msg *pfcp.Message, pfcpErr error) {
	logger.PfcpLog.Debugf("send association setup request error [%v] ", pfcpErr.Error())
	// send Error
	sendErrRsp(msg, pfcpErr)
}

func handleSendPfcpHeartbeatReqError(msg *pfcp.Message, pfcpErr error) {
	logger.PfcpLog.Debugf("send heartbeat request error [%v]", pfcpErr.Error())
	// send Error
	sendErrRsp(msg, pfcpErr)
}

func handleSendPfcpSessEstReqError(msg *pfcp.Message, pfcpErr error) {
	logger.PfcpLog.Debugf("send session establishment request error [%v] ", pfcpErr.Error())
	// send Error
	sendErrRsp(msg, pfcpErr)
}

func handleSendPfcpSessModReqError(msg *pfcp.Message, pfcpErr error) {
	logger.PfcpLog.Debugf("send session modification request error [%v]", pfcpErr.Error())
	// send Error
	sendErrRsp(msg, pfcpErr)
}

func handleSendPfcpSessRelReqError(msg *pfcp.Message, pfcpErr error) {
	logger.PfcpLog.Debugf("send session release request error [%v]", pfcpErr.Error())
	// send Error
	sendErrRsp(msg, pfcpErr)
}

func sendErrRsp(msg *pfcp.Message, err error) {
	// Get the PFCP Txn
	pfcpTxnChan := config.GetUpfPfcpTxn(msg.Header.SequenceNumber)

	// Send Rsp back to http txn
	pfcpTxnChan <- config.PfcpHttpRsp{Rsp: nil, Err: err}
}

func encodeAndSendRsp(msg *pfcpUdp.Message) error {
	pfcpHttpRsp := config.PfcpHttpRsp{}
	pRspJson, err := json.Marshal(msg.PfcpMessage)
	if err != nil {
		pfcpHttpRsp.Rsp = nil
		pfcpHttpRsp.Err = err
		logger.AppLog.Errorf("encodeAndSendRsp, json encode error [%v] ", err)
		return err
	}

	// Get the PFCP Txn
	pfcpTxnChan := config.GetUpfPfcpTxn(msg.PfcpMessage.Header.SequenceNumber)

	// Send Rsp back to http txn
	pfcpTxnChan <- config.PfcpHttpRsp{Rsp: pRspJson, Err: nil}

	return nil
}

func HandlePfcpAssociationSetupResponse(msg *pfcpUdp.Message) {
	pMsgBody := msg.PfcpMessage.Body.(pfcp.PFCPAssociationSetupResponse)

	logger.PfcpLog.Debugf("HandlePfcpAssociationSetupResponse, recovery timestamp [%v] ", pMsgBody.RecoveryTimeStamp)

	if pMsgBody.Cause.CauseValue == pfcpType.CauseRequestAccepted {

		// UPF's node ID
		nodeId := pMsgBody.NodeID
		// Add UPF as active
		logger.PfcpLog.Debugf("node id from pfcp association response [%v] ", nodeId)
		upf := config.ActivateUpfNode(nodeId)

		// Preserve success Asso Rsp
		upf.PreservePfcpAssociationRsp(pMsgBody)
	}

	msg.PfcpMessage.Body = pMsgBody

	// Encode pfcp rsp to json and send to http txn
	if err := encodeAndSendRsp(msg); err != nil {
		logger.PfcpLog.Errorf("handle pfcp association response error [%v] ", err)
	}
}

func HandlePfcpHeartbeatResponse(msg *pfcpUdp.Message) {
	pMsgBody := msg.PfcpMessage.Body.(pfcp.HeartbeatResponse)
	logger.PfcpLog.Debugf("pfcp heartbeat response recovery timestamp [%v] ", pMsgBody.RecoveryTimeStamp)
	msg.PfcpMessage.Body = pMsgBody
	// Encode pfcp rsp to json and send to http txn
	if err := encodeAndSendRsp(msg); err != nil {
		logger.PfcpLog.Errorf("handle pfcp heartbeat response error [%v] ", err)
	}
}

func HandlePfcpSessionEstablishmentResponse(msg *pfcpUdp.Message) {
	pMsgBody := msg.PfcpMessage.Body.(pfcp.PFCPSessionEstablishmentResponse)

	msg.PfcpMessage.Body = pMsgBody
	// Encode pfcp rsp to json and send to http txn
	if err := encodeAndSendRsp(msg); err != nil {
		logger.PfcpLog.Errorf("handle pfcp session establishment response error [%v] ", err)
	}
}

func HandlePfcpSessionModificationResponse(msg *pfcpUdp.Message) {
	pMsgBody := msg.PfcpMessage.Body.(pfcp.PFCPSessionModificationResponse)
	msg.PfcpMessage.Body = pMsgBody
	// Encode pfcp rsp to json and send to http txn
	if err := encodeAndSendRsp(msg); err != nil {
		logger.PfcpLog.Errorf("handle pfcp session modify response error [%v] ", err)
	}
}

func HandlePfcpSessionDeletionResponse(msg *pfcpUdp.Message) {
	pMsgBody := msg.PfcpMessage.Body.(pfcp.PFCPSessionDeletionResponse)
	msg.PfcpMessage.Body = pMsgBody
	// Encode pfcp rsp to json and send to http txn
	if err := encodeAndSendRsp(msg); err != nil {
		logger.PfcpLog.Errorf("handle pfcp session delete response error [%v] ", err)
	}
}

func BuildPfcpAssociationResponse(nodeId *pfcpType.NodeID, seqNo uint32) (*pfcp.Message, error) {
	logger.AppLog.Debugf("building pfcp association response for upf [%v], seqNo [%v]", nodeId, seqNo)
	if upf := config.GetUpfFromNodeId(nodeId); upf != nil {
		pfcpMsg := upf.LastAssoRsp.(pfcp.PFCPAssociationSetupResponse)

		logger.AppLog.Debugf("stored association response recovery timestamp: %v", pfcpMsg.RecoveryTimeStamp)

		message := pfcp.Message{
			Header: pfcp.Header{
				Version:        pfcp.PfcpVersion,
				MP:             0,
				S:              pfcp.SEID_NOT_PRESENT,
				MessageType:    pfcp.PFCP_ASSOCIATION_SETUP_RESPONSE,
				SequenceNumber: seqNo,
			},
			Body: pfcpMsg,
		}

		return &message, nil
	}

	logger.AppLog.Errorf("upf [%v] not found", string(nodeId.NodeIdValue))
	return nil, fmt.Errorf("upf not found : %v", string(nodeId.NodeIdValue))
}

func BuildPfcpHeartBeatResponse(nodeId *pfcpType.NodeID, seqNo uint32) (*pfcp.Message, error) {
	logger.PfcpLog.Debugf("building pfcp heartbeat response for upf:[%v], seqNo:[%v]", nodeId, seqNo)
	if upf := config.GetUpfFromNodeId(nodeId); upf != nil {
		pfcpMsg := upf.LastHBRsp.(pfcp.HeartbeatResponse)

		logger.PfcpLog.Debugf("stored heartbeat rsp recovery timestamp: %v", pfcpMsg.RecoveryTimeStamp)

		message := pfcp.Message{
			Header: pfcp.Header{
				Version:        pfcp.PfcpVersion,
				MP:             0,
				S:              pfcp.SEID_NOT_PRESENT,
				MessageType:    pfcp.PFCP_HEARTBEAT_RESPONSE,
				SequenceNumber: seqNo,
			},
			Body: pfcpMsg,
		}

		return &message, nil
	}

	return nil, fmt.Errorf("upf not found : %v", string(nodeId.NodeIdValue))
}
