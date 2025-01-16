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
	upfPort uint16,
) (bool, *context.PFCPSessionResponseStatus, error) {
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return false, nil, fmt.Errorf("PFCP Context not found for NodeID[%v]", upNodeID)
	}

	nodeIDIPAddress := context.SMF_Self().CPNodeID.ResolveNodeIdToIp()

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
		return false, nil, err
	}
	rsp, err := upf.HandlePfcpSessionEstablishmentRequest(pfcpMsg)
	if err != nil {
		return false, nil, fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	addPduSessionAnchor, status, err := HandlePfcpSessionEstablishmentRequest(rsp)
	if err != nil {
		return false, status, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return addPduSessionAnchor, status, nil
}

func HandlePfcpSessionEstablishmentRequest(msg *message.SessionEstablishmentResponse) (bool, *context.PFCPSessionResponseStatus, error) {
	var addPduSessionAnchor bool
	SEID := msg.SEID()
	smContext := context.GetSMContextBySEID(SEID)
	if smContext == nil {
		return addPduSessionAnchor, nil, fmt.Errorf("failed to find SMContext for SEID[%d]", SEID)
	}
	smContext.SMLock.Lock()

	if msg.NodeID == nil {
		return addPduSessionAnchor, nil, fmt.Errorf("PFCP Session Establishment Response missing NodeID")
	}
	nodeID, err := msg.NodeID.NodeID()
	if err != nil {
		return addPduSessionAnchor, nil, fmt.Errorf("failed to parse NodeID IE: %+v", err)
	}

	if msg.UPFSEID != nil {
		pfcpSessionCtx := smContext.PFCPContext[nodeID]
		rspUPFseid, err := msg.UPFSEID.FSEID()
		if err != nil {
			return addPduSessionAnchor, nil, fmt.Errorf("failed to parse FSEID IE: %+v", err)
		}
		pfcpSessionCtx.RemoteSEID = rspUPFseid.SEID
		smContext.SubPfcpLog.Infof("in HandlePfcpSessionEstablishmentResponse rsp.UPFSEID.Seid [%v] ", rspUPFseid.SEID)
	}

	// Get N3 interface UPF
	ANUPF := smContext.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode

	// UE IP-Addr(only v4 supported)
	if msg.CreatedPDR != nil {
		ueIPAddress := FindUEIPAddress(msg.CreatedPDR)
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
		fteid, err := FindFTEID(msg.CreatedPDR)
		if err != nil {
			return addPduSessionAnchor, nil, fmt.Errorf("failed to parse TEID IE: %+v", err)
		}
		logger.SmfLog.Infof("created PDR FTEID: %+v", fteid)
		ANUPF.UpLinkTunnel.TEID = fteid.TEID
		upf := context.GetUserPlaneInformation().UPF.UPF
		if upf == nil {
			return addPduSessionAnchor, nil, fmt.Errorf("can't find UPF[%s]", nodeID)
		}
		upf.N3Interfaces = make([]context.UPFInterfaceInfo, 0)
		n3Interface := context.UPFInterfaceInfo{}
		n3Interface.IPv4EndPointAddresses = append(n3Interface.IPv4EndPointAddresses, fteid.IPv4Address)
		upf.N3Interfaces = append(upf.N3Interfaces, n3Interface)
	}
	smContext.SMLock.Unlock()

	if msg.NodeID == nil {
		return addPduSessionAnchor, nil, fmt.Errorf("PFCP Session Establishment Response missing NodeID")
	}

	if msg.Cause == nil {
		return addPduSessionAnchor, nil, fmt.Errorf("PFCP Session Establishment Response missing Cause")
	}
	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return addPduSessionAnchor, nil, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}
	var status context.PFCPSessionResponseStatus
	if causeValue == ie.CauseRequestAccepted {
		status = context.SessionEstablishSuccess
		smContext.SubPfcpLog.Infof("PFCP Session Establishment accepted")
	} else {
		status = context.SessionEstablishFailed
		smContext.SubPfcpLog.Errorf("PFCP Session Establishment rejected with cause [%v]", causeValue)
	}

	if context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == context.AddingPSA {
			smContext.SubPfcpLog.Infoln("keep Adding PSAndULCL")
			addPduSessionAnchor = true
			smContext.BPManager.BPStatus = context.AddingPSA
		}
	}
	return addPduSessionAnchor, &status, nil
}

func HandlePfcpSessionModificationResponse(msg *message.SessionModificationResponse) (bool, *context.PFCPSessionResponseStatus, error) {
	var addPduSessionAnchor bool
	SEID := msg.SEID()

	smContext := context.GetSMContextBySEID(SEID)

	if context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
		if smContext.BPManager.BPStatus == context.AddingPSA {
			addPduSessionAnchor = true
		}
	}

	if msg.Cause == nil {
		return addPduSessionAnchor, nil, fmt.Errorf("PFCP Session Modification Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return addPduSessionAnchor, nil, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	var status context.PFCPSessionResponseStatus
	if causeValue == ie.CauseRequestAccepted {
		smContext.SubPduSessLog.Infoln("PFCP Modification Response Accept")
		if smContext.SMContextState == context.SmStatePfcpModify {
			upfNodeID := smContext.GetNodeIDByLocalSEID(SEID)
			upfIP := upfNodeID.ResolveNodeIdToIp().String()
			delete(smContext.PendingUPF, upfIP)
			smContext.SubPduSessLog.Debugf("Delete pending pfcp response: UPF IP [%s]\n", upfIP)

			if smContext.PendingUPF.IsEmpty() {
				status = context.SessionUpdateSuccess
			}

			if context.SMF_Self().ULCLSupport && smContext.BPManager != nil {
				if smContext.BPManager.BPStatus == context.UnInitialized {
					smContext.BPManager.BPStatus = context.AddingPSA
					addPduSessionAnchor = true
				}
			}
		}

		smContext.SubPfcpLog.Infof("PFCP Session Modification Success[%d]\n", SEID)
	} else {
		smContext.SubPfcpLog.Infof("PFCP Session Modification Failed[%d]\n", SEID)
		if smContext.SMContextState == context.SmStatePfcpModify {
			status = context.SessionUpdateFailed
		}
	}

	smContext.SubCtxLog.Debugln("PFCP Session Context")
	for _, ctx := range smContext.PFCPContext {
		smContext.SubCtxLog.Debugln(ctx.String())
	}
	return addPduSessionAnchor, &status, nil
}

func SendPfcpSessionModificationRequest(
	upNodeID context.NodeID,
	ctx *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
	upfPort uint16,
) (bool, *context.PFCPSessionResponseStatus, error) {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return false, nil, fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg, err := BuildPfcpSessionModificationRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMF_Self().CPNodeID.ResolveNodeIdToIp(), pdrList, farList, qerList)
	if err != nil {
		return false, nil, fmt.Errorf("failed to build PFCP Session Modification Request: %v", err)
	}
	rsp, err := upf.HandlePfcpSessionModificationRequest(pfcpMsg)
	if err != nil {
		return false, nil, fmt.Errorf("failed to handle PFCP Session Establishment Request in upf: %v", err)
	}
	addPduSessionAnchor, status, err := HandlePfcpSessionModificationResponse(rsp)
	if err != nil {
		return false, status, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}
	return addPduSessionAnchor, status, nil
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

func SendPfcpSessionDeletionRequest(upNodeID context.NodeID, ctx *context.SMContext, upfPort uint16) (*context.PFCPSessionResponseStatus, error) {
	seqNum := getSeqNumber()
	upNodeIDStr := upNodeID.ResolveNodeIdToIp().String()
	pfcpContext, ok := ctx.PFCPContext[upNodeIDStr]
	if !ok {
		return nil, fmt.Errorf("PFCP Context not found for NodeID[%s]", upNodeIDStr)
	}
	pfcpMsg := BuildPfcpSessionDeletionRequest(seqNum, pfcpContext.LocalSEID, pfcpContext.RemoteSEID, context.SMF_Self().CPNodeID.ResolveNodeIdToIp())

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
