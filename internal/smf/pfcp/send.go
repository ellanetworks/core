// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	"fmt"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	upf "github.com/ellanetworks/core/internal/upf/core"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

var seq uint32

func getSeqNumber() uint32 {
	return atomic.AddUint32(&seq, 1)
}

func SendPfcpSessionEstablishmentRequest(
	upNodeID context.NodeID,
	ctx *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
) (*context.PFCPSessionResponseStatus, error) {
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return nil, fmt.Errorf("PFCP Context not found for NodeID[%v]", upNodeID)
	}

	nodeIDIPAddress := context.SMFSelf().CPNodeID.ResolveNodeIDToIP()

	pfcpMsg, err := BuildPfcpSessionEstablishmentRequest(
		getSeqNumber(),
		nodeIDIPAddress.String(),
		nodeIDIPAddress,
		pfcpContext.LocalSEID,
		pdrList,
		farList,
		qerList,
	)
	if err != nil {
		return nil, err
	}
	rsp, err := upf.HandlePfcpSessionEstablishmentRequest(pfcpMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	status, err := HandlePfcpSessionEstablishmentRequest(rsp)
	if err != nil {
		return status, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return status, nil
}

func HandlePfcpSessionEstablishmentRequest(msg *message.SessionEstablishmentResponse) (*context.PFCPSessionResponseStatus, error) {
	SEID := msg.SEID()
	smContext := context.GetSMContextBySEID(SEID)
	if smContext == nil {
		return nil, fmt.Errorf("failed to find SMContext for SEID[%d]", SEID)
	}
	smContext.SMLock.Lock()

	if msg.NodeID == nil {
		return nil, fmt.Errorf("PFCP Session Establishment Response missing NodeID")
	}
	nodeID, err := msg.NodeID.NodeID()
	if err != nil {
		return nil, fmt.Errorf("failed to parse NodeID IE: %+v", err)
	}

	if msg.UPFSEID != nil {
		pfcpSessionCtx := smContext.PFCPContext[nodeID]
		rspUPFseid, err := msg.UPFSEID.FSEID()
		if err != nil {
			return nil, fmt.Errorf("failed to parse FSEID IE: %+v", err)
		}
		pfcpSessionCtx.RemoteSEID = rspUPFseid.SEID
	}

	// Get N3 interface UPF
	ANUPF := smContext.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode

	// UE IP-Addr(only v4 supported)
	if msg.CreatedPDR != nil {
		ueIPAddress := FindUEIPAddress(msg.CreatedPDR)
		if ueIPAddress != nil {
			smContext.SubPfcpLog.Infof("upf provided ue ip address [%v]", ueIPAddress)
			// Release previous locally allocated UE IP-Addr
			err := smContext.ReleaseUeIPAddr()
			if err != nil {
				logger.SmfLog.Errorf("failed to release UE IP-Addr: %+v", err)
			}

			// Update with one received from UPF
			smContext.PDUAddress.IP = ueIPAddress
			smContext.PDUAddress.UpfProvided = true
		}

		// Store F-TEID created by UPF
		fteid, err := FindFTEID(msg.CreatedPDR)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TEID IE: %+v", err)
		}
		logger.SmfLog.Debugf("Created PDR F-TEID: %+v", fteid)
		ANUPF.UpLinkTunnel.TEID = fteid.TEID
		upf := context.GetUserPlaneInformation().UPF.UPF
		if upf == nil {
			return nil, fmt.Errorf("can't find UPF[%s]", nodeID)
		}
		upf.N3Interfaces = make([]context.UPFInterfaceInfo, 0)
		n3Interface := context.UPFInterfaceInfo{}
		n3Interface.IPv4EndPointAddresses = append(n3Interface.IPv4EndPointAddresses, fteid.IPv4Address)
		upf.N3Interfaces = append(upf.N3Interfaces, n3Interface)
	}
	smContext.SMLock.Unlock()

	if msg.NodeID == nil {
		return nil, fmt.Errorf("PFCP Session Establishment Response missing NodeID")
	}

	if msg.Cause == nil {
		return nil, fmt.Errorf("PFCP Session Establishment Response missing Cause")
	}
	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}
	var status context.PFCPSessionResponseStatus
	if causeValue == ie.CauseRequestAccepted {
		status = context.SessionEstablishSuccess
		smContext.SubPfcpLog.Infof("PFCP Session Establishment accepted")
	} else {
		status = context.SessionEstablishFailed
		smContext.SubPfcpLog.Errorf("PFCP Session Establishment rejected with cause [%v]", causeValue)
	}

	return &status, nil
}

func HandlePfcpSessionModificationResponse(msg *message.SessionModificationResponse) (*context.PFCPSessionResponseStatus, error) {
	SEID := msg.SEID()

	smContext := context.GetSMContextBySEID(SEID)

	if msg.Cause == nil {
		return nil, fmt.Errorf("PFCP Session Modification Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		status := context.SessionUpdateFailed
		return &status, fmt.Errorf("PFCP Session Modification Failed[%d]", SEID)
	}
	smContext.SubPduSessLog.Debugln("PFCP Modification Response Accept")
	upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
	upfIP := upfNodeID.ResolveNodeIDToIP().String()
	delete(smContext.PendingUPF, upfIP)
	smContext.SubPduSessLog.Debugf("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

	var status context.PFCPSessionResponseStatus
	if smContext.PendingUPF.IsEmpty() {
		status = context.SessionUpdateSuccess
	}
	smContext.SubPfcpLog.Debugf("PFCP Session Modification Success[%d]\n", SEID)
	return &status, nil
}

func SendPfcpSessionModificationRequest(
	upNodeID context.NodeID,
	ctx *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
) (*context.PFCPSessionResponseStatus, error) {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return nil, fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg, err := BuildPfcpSessionModificationRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMFSelf().CPNodeID.ResolveNodeIDToIP(), pdrList, farList, qerList)
	if err != nil {
		return nil, fmt.Errorf("failed to build PFCP Session Modification Request: %v", err)
	}
	rsp, err := upf.HandlePfcpSessionModificationRequest(pfcpMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	status, err := HandlePfcpSessionModificationResponse(rsp)
	if err != nil {
		return status, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return status, nil
}

func HandlePfcpSessionDeletionResponse(msg *message.SessionDeletionResponse) (*context.PFCPSessionResponseStatus, error) {
	if msg.Cause == nil {
		return nil, fmt.Errorf("PFCP Session Deletion Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	var status context.PFCPSessionResponseStatus
	if causeValue == ie.CauseRequestAccepted {
		status = context.SessionReleaseSuccess
	} else {
		status = context.SessionReleaseFailed
	}

	return &status, nil
}

func SendPfcpSessionDeletionRequest(upNodeID context.NodeID, ctx *context.SMContext) (*context.PFCPSessionResponseStatus, error) {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return nil, fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg := BuildPfcpSessionDeletionRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMFSelf().CPNodeID.ResolveNodeIDToIP())

	rsp, err := upf.HandlePfcpSessionDeletionRequest(pfcpMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	status, err := HandlePfcpSessionDeletionResponse(rsp)
	if err != nil {
		return status, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return status, nil
}
