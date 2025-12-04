// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	ctxt "context"
	"fmt"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"go.uber.org/zap"
)

var seq uint32

var dispatcher *pfcp_dispatcher.PfcpDispatcher = &pfcp_dispatcher.Dispatcher

func getSeqNumber() uint32 {
	return atomic.AddUint32(&seq, 1)
}

func SendPfcpSessionEstablishmentRequest(
	ctx ctxt.Context,
	upNodeID context.NodeID,
	smCtx *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
	urrList []*context.URR,
) error {
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := smCtx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP context not found for Node ID: %v", upNodeID)
	}

	nodeIDIPAddress := context.SMFSelf().CPNodeID

	pfcpMsg, err := BuildPfcpSessionEstablishmentRequest(
		getSeqNumber(),
		nodeIDIPAddress.String(),
		nodeIDIPAddress,
		pfcpContext.LocalSEID,
		pdrList,
		farList,
		qerList,
		urrList,
	)
	if err != nil {
		return fmt.Errorf("failed to build PFCP Session Establishment Request: %v", err)
	}
	rsp, err := dispatcher.UPF.HandlePfcpSessionEstablishmentRequest(ctx, pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to send PFCP Session Establishment Request to upf: %v", err)
	}
	err = HandlePfcpSessionEstablishmentResponse(ctx, rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return nil
}

func HandlePfcpSessionEstablishmentResponse(ctx ctxt.Context, msg *message.SessionEstablishmentResponse) error {
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
	ANUPF := dataPath.DPNode
	smfSelf := context.SMFSelf()

	// UE IP-Addr(only v4 supported)
	if msg.CreatedPDR != nil {
		ueIPAddress := FindUEIPAddress(msg.CreatedPDR)
		if ueIPAddress != nil {
			smContext.SubPfcpLog.Info("upf provided ue ip address", zap.String("IP", ueIPAddress.String()))
			// Release previous locally allocated UE IP-Addr
			err := smContext.ReleaseUeIPAddr(ctx)
			if err != nil {
				return fmt.Errorf("failed to release UE IP-Addr: %+v", err)
			}

			// Update with one received from UPF
			smContext.PDUAddress = ueIPAddress
		}

		// Store F-TEID created by UPF
		fteid, err := FindFTEID(msg.CreatedPDR)
		if err != nil {
			return fmt.Errorf("failed to parse TEID IE: %+v", err)
		}
		ANUPF.UpLinkTunnel.TEID = fteid.TEID
		upf := smfSelf.UPF
		if smfSelf.UPF == nil {
			return fmt.Errorf("can't find UPF: %s", nodeID)
		}

		upf.N3Interface = fteid.IPv4Address
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
		smContext.SubPfcpLog.Info("PFCP Session Establishment accepted")
		return nil
	}
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

	smContext.SubPfcpLog.Debug("PFCP Session Modification Success", zap.Uint64("SEID", SEID))

	return nil
}

func SendPfcpSessionModificationRequest(
	ctx ctxt.Context,
	upNodeID context.NodeID,
	smCtx *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
) error {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := smCtx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg, err := BuildPfcpSessionModificationRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMFSelf().CPNodeID, pdrList, farList, qerList)
	if err != nil {
		return fmt.Errorf("failed to build PFCP Session Modification Request: %v", err)
	}
	rsp, err := dispatcher.UPF.HandlePfcpSessionModificationRequest(ctx, pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to send PFCP Session Establishment Request to upf: %v", err)
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

func SendPfcpSessionDeletionRequest(ctx ctxt.Context, upNodeID context.NodeID, smCtx *context.SMContext) error {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIDToIP().String()
	pfcpContext, ok := smCtx.PFCPContext[upNodeIDStr]
	if !ok {
		return fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg := BuildPfcpSessionDeletionRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMFSelf().CPNodeID)
	rsp, err := dispatcher.UPF.HandlePfcpSessionDeletionRequest(ctx, pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to send PFCP Session Establishment Request to upf: %v", err)
	}
	err = HandlePfcpSessionDeletionResponse(rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return nil
}
