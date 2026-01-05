// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
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
	ctx context.Context,
	smf *smfContext.SMF,
	localSEID uint64,
	pdrList []*smfContext.PDR,
	farList []*smfContext.FAR,
	qerList []*smfContext.QER,
	urrList []*smfContext.URR,
) error {
	pfcpMsg, err := BuildPfcpSessionEstablishmentRequest(
		getSeqNumber(),
		smf.CPNodeID.String(),
		smf.CPNodeID,
		localSEID,
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

	err = HandlePfcpSessionEstablishmentResponse(ctx, smf, rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}

	return nil
}

func HandlePfcpSessionEstablishmentResponse(ctx context.Context, smf *smfContext.SMF, msg *message.SessionEstablishmentResponse) error {
	seid := msg.SEID()

	smContext := smf.GetSMContextBySEID(seid)
	if smContext == nil {
		return fmt.Errorf("failed to find SM Context for SEID: %d", seid)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if msg.NodeID == nil {
		return fmt.Errorf("PFCP Session Establishment Response missing Node ID")
	}

	if msg.UPFSEID != nil {
		rspUPFseid, err := msg.UPFSEID.FSEID()
		if err != nil {
			return fmt.Errorf("failed to parse FSEID IE: %+v", err)
		}

		smContext.PFCPContext.RemoteSEID = rspUPFseid.SEID
	}

	// UE IP-Addr(only v4 supported)
	if msg.CreatedPDR != nil {
		fteid, err := findFTEID(msg.CreatedPDR)
		if err != nil {
			return fmt.Errorf("failed to parse TEID IE: %+v", err)
		}

		smContext.Tunnel.DataPath.UpLinkTunnel.TEID = fteid.TEID
		smContext.Tunnel.DataPath.UpLinkTunnel.N3IP = fteid.IPv4Address
	}

	if msg.Cause == nil {
		return fmt.Errorf("PFCP Session Establishment Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	if causeValue == ie.CauseRequestAccepted {
		logger.SmfLog.Info("PFCP Session Establishment accepted", zap.Uint64("SEID", seid), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		return nil
	}

	return fmt.Errorf("PFCP Session Establishment rejected with cause: %v", causeValue)
}

func HandlePfcpSessionModificationResponse(msg *message.SessionModificationResponse) error {
	if msg.Cause == nil {
		return fmt.Errorf("PFCP Session Modification Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	seid := msg.SEID()

	if causeValue != ie.CauseRequestAccepted {
		return fmt.Errorf("PFCP Session Modification Failed: %d", seid)
	}

	logger.SmfLog.Info("PFCP Session Modification Success", zap.Uint64("SEID", seid))

	return nil
}

func SendPfcpSessionModificationRequest(
	ctx context.Context,
	cpNodeID net.IP,
	localSEID uint64,
	remoteSEID uint64,
	pdrList []*smfContext.PDR,
	farList []*smfContext.FAR,
	qerList []*smfContext.QER,
) error {
	seqNum := getSeqNumber()

	pfcpMsg, err := BuildPfcpSessionModificationRequest(seqNum, localSEID, remoteSEID, cpNodeID, pdrList, farList, qerList)
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

func SendPfcpSessionDeletionRequest(ctx context.Context, cpNodeID net.IP, localSEID uint64, remoteSEID uint64) error {
	seqNum := getSeqNumber()

	pfcpMsg := BuildPfcpSessionDeletionRequest(seqNum, localSEID, remoteSEID, cpNodeID)

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
