// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/wmnsk/go-pfcp/message"
)

var seq uint32

var dispatcher *pfcp_dispatcher.PfcpDispatcher = &pfcp_dispatcher.Dispatcher

func getSeqNumber() uint32 {
	return atomic.AddUint32(&seq, 1)
}

func SendPfcpSessionReportRequest(ctx context.Context, seid uint64, pdrid uint16, qfi uint8) error {
	pfcpMsg, err := BuildPfcpSessionReportRequest(seid, getSeqNumber(), pdrid, qfi)
	if err != nil {
		return fmt.Errorf("failed to build PFCP Session Report Request: %v", err)
	}
	rsp, err := dispatcher.SMF.HandlePfcpSessionReportRequest(ctx, pfcpMsg)
	if err != nil {
		return fmt.Errorf("failed to send PFCP Session Report Request to smf: %v", err)
	}
	err = HandlePfcpSessionReportResponse(ctx, rsp)
	if err != nil {
		return fmt.Errorf("failed to handle PFCP Session Report Response: %v", err)
	}
	return nil
}

func HandlePfcpSessionReportResponse(ctx context.Context, rsp *message.SessionReportResponse) error {
	return nil
}
