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
	"strings"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

var seq uint32

var dispatcher *pfcp_dispatcher.PfcpDispatcher = &pfcp_dispatcher.Dispatcher

func getSeqNumber() uint32 {
	return atomic.AddUint32(&seq, 1)
}

func extractImsiFromSupi(supi string) string {
	return strings.TrimPrefix(supi, "imsi-")
}

type PFCPSessionEstablishmentResult struct {
	RemoteSEID uint64
	TEID       uint32
	N3IP       net.IP
}

func SendPfcpSessionEstablishmentRequest(
	ctx context.Context,
	cpNodeID net.IP,
	localSEID uint64,
	pdrList []*smfContext.PDR,
	farList []*smfContext.FAR,
	qerList []*smfContext.QER,
	urrList []*smfContext.URR,
	supi string,
) (*PFCPSessionEstablishmentResult, error) {
	imsi := extractImsiFromSupi(supi)

	pfcpMsg, err := BuildPfcpSessionEstablishmentRequest(
		getSeqNumber(),
		cpNodeID.String(),
		cpNodeID,
		localSEID,
		pdrList,
		farList,
		qerList,
		urrList,
		imsi,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build PFCP Session Establishment Request: %v", err)
	}

	rsp, err := dispatcher.UPF.HandlePfcpSessionEstablishmentRequest(ctx, pfcpMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to send PFCP Session Establishment Request to upf: %v", err)
	}

	result, err := HandlePfcpSessionEstablishmentResponse(rsp)
	if err != nil {
		return nil, fmt.Errorf("failed to handle PFCP Session Establishment Response: %v", err)
	}

	return result, nil
}

func HandlePfcpSessionEstablishmentResponse(msg *message.SessionEstablishmentResponse) (*PFCPSessionEstablishmentResult, error) {
	if msg.UPFSEID == nil {
		return nil, fmt.Errorf("PFCP Session Establishment Response missing UPF SEID")
	}

	rspUPFseid, err := msg.UPFSEID.FSEID()
	if err != nil {
		return nil, fmt.Errorf("failed to parse FSEID IE: %+v", err)
	}

	if msg.CreatedPDR == nil {
		return nil, fmt.Errorf("PFCP Session Establishment Response missing Created PDR")
	}

	fteid, err := findFTEID(msg.CreatedPDR)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TEID IE: %+v", err)
	}

	if msg.Cause == nil {
		return nil, fmt.Errorf("PFCP Session Establishment Response missing Cause")
	}

	causeValue, err := msg.Cause.Cause()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Cause IE: %+v", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		return nil, fmt.Errorf("PFCP Session Establishment Failed: %v", causeValue)
	}

	result := &PFCPSessionEstablishmentResult{
		RemoteSEID: rspUPFseid.SEID,
		TEID:       fteid.TEID,
		N3IP:       fteid.IPv4Address,
	}

	return result, nil
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

func findFTEID(createdPDRIEs []*ie.IE) (*ie.FTEIDFields, error) {
	for _, createdPDRIE := range createdPDRIEs {
		teid, err := createdPDRIE.FTEID()
		if err == nil {
			return teid, nil
		}
	}

	return nil, fmt.Errorf("FTEID not found in CreatedPDR")
}
