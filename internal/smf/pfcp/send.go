// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	"fmt"
	"sync/atomic"

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
) error {
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP context not found for Node ID: %v", upNodeID)
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
		return fmt.Errorf("failed to build PFCP Session Establishment Request: %v", err)
	}
	rsp, err := upf.HandlePfcpSessionEstablishmentRequest(pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	err = HandlePfcpSessionEstablishmentResponse(rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return nil
}

func HandlePfcpSessionEstablishmentResponse(msg *message.SessionEstablishmentResponse) error {
	SEID := msg.SEID()
	smContext := context.GetSMContextBySEID(SEID)
	if smContext == nil {
		return fmt.Errorf("failed to find SM Context for SEID: %d", SEID)
	}
	smContext.SMLock.Lock()

	if msg.NodeID == nil {
		return fmt.Errorf("PFCP Session Establishment Response missing Node ID")
	}
	nodeID, err := msg.NodeID.NodeID()
	if err != nil {
		return fmt.Errorf("failed to parse NodeID IE: %+v", err)
	}

	if msg.UPFSEID != nil {
		pfcpSessionCtx := smContext.PFCPContext[nodeID]
		rspUPFseid, err := msg.UPFSEID.FSEID()
		if err != nil {
			return fmt.Errorf("failed to parse FSEID IE: %+v", err)
		}
		pfcpSessionCtx.RemoteSEID = rspUPFseid.SEID
	}

	// Get N3 interface UPF
	dataPath := smContext.Tunnel.DataPath
	ANUPF := dataPath.FirstDPNode

	// UE IP-Addr(only v4 supported)
	if msg.CreatedPDR != nil {
		ueIPAddress := FindUEIPAddress(msg.CreatedPDR)
		if ueIPAddress != nil {
			smContext.SubPfcpLog.Infof("upf provided ue ip address [%v]", ueIPAddress)
			// Release previous locally allocated UE IP-Addr
			err := smContext.ReleaseUeIPAddr()
			if err != nil {
				return fmt.Errorf("failed to release UE IP-Addr: %+v", err)
			}

			// Update with one received from UPF
			smContext.PDUAddress.IP = ueIPAddress
			smContext.PDUAddress.UpfProvided = true
		}

		// Store F-TEID created by UPF
		fteid, err := FindFTEID(msg.CreatedPDR)
		if err != nil {
			return fmt.Errorf("failed to parse TEID IE: %+v", err)
		}
		ANUPF.UpLinkTunnel.TEID = fteid.TEID
		upf := context.GetUserPlaneInformation()
		if upf == nil {
			return fmt.Errorf("can't find UPF: %s", nodeID)
		}
		upf.N3Interfaces = make([]context.UPFInterfaceInfo, 0)
		n3Interface := context.UPFInterfaceInfo{}
		n3Interface.IPv4EndPointAddresses = append(n3Interface.IPv4EndPointAddresses, fteid.IPv4Address)
		upf.N3Interfaces = append(upf.N3Interfaces, n3Interface)
	}
	smContext.SMLock.Unlock()

	if msg.Cause == nil {
		return fmt.Errorf("PFCP Session Establishment Response missing Cause")
	}
	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return fmt.Errorf("failed to parse Cause IE: %+v", err)
	}
	if causeValue == ie.CauseRequestAccepted {
		smContext.SubPfcpLog.Infof("PFCP Session Establishment accepted")
		return nil
	}
	smContext.SubPfcpLog.Errorf("PFCP Session Establishment rejected with cause [%v]", causeValue)
	return fmt.Errorf("PFCP Session Establishment rejected with cause: %v", causeValue)
}

func HandlePfcpSessionModificationResponse(msg *message.SessionModificationResponse) error {
	SEID := msg.SEID()

	smContext := context.GetSMContextBySEID(SEID)

	if msg.Cause == nil {
		return fmt.Errorf("PFCP Session Modification Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		return fmt.Errorf("PFCP Session Modification Failed: %d", SEID)
	}
	smContext.SubPduSessLog.Debugln("PFCP Modification Response Accept")
	upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
	upfIP := upfNodeID.ResolveNodeIDToIP().String()
	smContext.SubPduSessLog.Debugf("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

	smContext.SubPfcpLog.Debugf("PFCP Session Modification Success[%d]\n", SEID)
	return nil
}

func SendPfcpSessionModificationRequest(
	upNodeID context.NodeID,
	ctx *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
) error {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg, err := BuildPfcpSessionModificationRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMFSelf().CPNodeID.ResolveNodeIDToIP(), pdrList, farList, qerList)
	if err != nil {
		return fmt.Errorf("failed to build PFCP Session Modification Request: %v", err)
	}
	rsp, err := upf.HandlePfcpSessionModificationRequest(pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	err = HandlePfcpSessionModificationResponse(rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return nil
}

func HandlePfcpSessionDeletionResponse(msg *message.SessionDeletionResponse) error {
	if msg.Cause == nil {
		return fmt.Errorf("PFCP Session Deletion Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		return fmt.Errorf("PFCP Session Deletion Failed: %v", causeValue)
	}
	return nil
}

func SendPfcpSessionDeletionRequest(upNodeID context.NodeID, ctx *context.SMContext) error {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg := BuildPfcpSessionDeletionRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMFSelf().CPNodeID.ResolveNodeIDToIP())

	rsp, err := upf.HandlePfcpSessionDeletionRequest(pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	err = HandlePfcpSessionDeletionResponse(rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return nil
}
