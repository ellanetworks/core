package pfcp

import (
	"encoding/json"
	"fmt"

	"upf-adapter/config"
	"upf-adapter/logger"
	"upf-adapter/pfcp/handler"
	"upf-adapter/pfcp/message"

	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpType"
)

func JsonBodyToPfcpAssocReq(body interface{}) pfcp.PFCPAssociationSetupRequest {
	jsonString, _ := json.Marshal(body)

	s := pfcp.PFCPAssociationSetupRequest{}
	json.Unmarshal(jsonString, &s)

	return s
}

func JsonBodyToPfcpHeartbeatReq(body interface{}) pfcp.HeartbeatRequest {
	jsonString, _ := json.Marshal(body)

	s := pfcp.HeartbeatRequest{}
	json.Unmarshal(jsonString, &s)

	return s
}

func JsonBodyToPfcpSessEstReq(body interface{}) pfcp.PFCPSessionEstablishmentRequest {
	jsonString, _ := json.Marshal(body)

	s := pfcp.PFCPSessionEstablishmentRequest{}
	json.Unmarshal(jsonString, &s)

	return s
}

func JsonBodyToPfcpSessModReq(body interface{}) pfcp.PFCPSessionModificationRequest {
	jsonString, _ := json.Marshal(body)

	s := pfcp.PFCPSessionModificationRequest{}
	json.Unmarshal(jsonString, &s)

	return s
}

func JsonBodyToPfcpSessDelReq(body interface{}) pfcp.PFCPSessionDeletionRequest {
	jsonString, _ := json.Marshal(body)

	s := pfcp.PFCPSessionDeletionRequest{}
	json.Unmarshal(jsonString, &s)

	return s
}

func ForwardPfcpMsgToUpf(udpPodMsg config.UdpPodPfcpMsg) ([]byte, error) {
	pMsg := udpPodMsg.Msg
	nodeId := udpPodMsg.UpNodeID
	pfcpTxnChan := make(config.PfcpTxnChan)
	var err error

	// identify msg type
	switch pMsg.Header.MessageType {
	case pfcp.PFCP_ASSOCIATION_SETUP_REQUEST:
		// if UPF is already associated then send Asso rsp

		if config.IsUpfAssociated(udpPodMsg.UpNodeID) {
			logger.AppLog.Debug("upf[%v] already associated", udpPodMsg.UpNodeID)
			// form and send Assoc rsp
			pfcpRsp, _ := handler.BuildPfcpAssociationResponse(&udpPodMsg.UpNodeID, pMsg.Header.SequenceNumber)
			pRspJson, _ := json.Marshal(pfcpRsp)

			return pRspJson, nil
		}

		config.InsertUpfNode(udpPodMsg.UpNodeID)

		// store txn in seq:chan map
		s := JsonBodyToPfcpAssocReq(pMsg.Body)

		// replace smf time-stamp with UPF-Adapters timestamp
		s.RecoveryTimeStamp.RecoveryTimeStamp = config.UpfServerStartTime

		logger.AppLog.Debugf("association request, existing node id[%v], replaced by [%v]", s.NodeID, config.UpfAdapterIp)

		// replace SMF NodeId with of upf-adapter's one
		s.NodeID = &pfcpType.NodeID{NodeIdType: pfcpType.NodeIdTypeIpv4Address, NodeIdValue: config.UpfAdapterIp}

		pMsg.Body = s
		config.InsertUpfPfcpTxn(pMsg.Header.SequenceNumber, pfcpTxnChan)
		err = message.SendPfcpAssociationSetupRequest(nodeId, pMsg)

	case pfcp.PFCP_HEARTBEAT_REQUEST:
		/*
			if config.IsUpfAssociated(udpPodMsg.UpNodeID) {
				logger.AppLog.Debug("upf[%v] already associated", udpPodMsg.UpNodeID)

				//form and send Heartbeat rsp
				pfcpRsp, _ := handler.BuildPfcpHeartBeatResponse(&udpPodMsg.UpNodeID, pMsg.Header.SequenceNumber)
				pRspJson, _ := json.Marshal(pfcpRsp)

				return pRspJson, nil
			}
		*/
		s := JsonBodyToPfcpHeartbeatReq(pMsg.Body)

		// replace smf time-stamp with UPF-Adapters timestamp
		s.RecoveryTimeStamp.RecoveryTimeStamp = config.UpfServerStartTime

		// store txn in seq:chan map
		config.InsertUpfPfcpTxn(pMsg.Header.SequenceNumber, pfcpTxnChan)
		pMsg.Body = s
		err = message.SendHeartbeatRequest(nodeId, pMsg)
	case pfcp.PFCP_SESSION_ESTABLISHMENT_REQUEST:
		s := JsonBodyToPfcpSessEstReq(pMsg.Body)
		// replace SMF NodeId with of upf-adapter's one
		s.NodeID = &pfcpType.NodeID{NodeIdType: pfcpType.NodeIdTypeIpv4Address, NodeIdValue: config.UpfAdapterIp}
		// Replace SMF FSEID v4 Addr with UPF Adapter's IP
		s.CPFSEID.Ipv4Address = config.UpfAdapterIp
		pMsg.Body = s
		// store txn in seq:chan map
		config.InsertUpfPfcpTxn(pMsg.Header.SequenceNumber, pfcpTxnChan)

		err = message.SendPfcpSessionEstablishmentRequest(nodeId, pMsg)
	case pfcp.PFCP_SESSION_MODIFICATION_REQUEST:
		s := JsonBodyToPfcpSessModReq(pMsg.Body)

		// store txn in seq:chan map
		config.InsertUpfPfcpTxn(pMsg.Header.SequenceNumber, pfcpTxnChan)
		pMsg.Body = s
		err = message.SendPfcpSessionModificationRequest(nodeId, pMsg)
	case pfcp.PFCP_SESSION_DELETION_REQUEST:
		s := JsonBodyToPfcpSessDelReq(pMsg.Body)

		// store txn in seq:chan map
		config.InsertUpfPfcpTxn(pMsg.Header.SequenceNumber, pfcpTxnChan)
		pMsg.Body = s
		message.SendPfcpSessionDeletionRequest(nodeId, pMsg)
	default:
		return nil, fmt.Errorf("invalid msg type [%v] from smf", pMsg.Header.MessageType)
	}

	if err != nil {
		return nil, err
	}

	// wait for response from UPF
	pfcpRsp := <-pfcpTxnChan

	return pfcpRsp.Rsp, pfcpRsp.Err
}
