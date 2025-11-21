// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

var seq uint32

var dispatcher *pfcp_dispatcher.PfcpDispatcher = &pfcp_dispatcher.Dispatcher

func getSeqNumber() uint32 {
	return atomic.AddUint32(&seq, 1)
}

func SendPfcpSessionReportRequest(ctx context.Context, localSeid uint64, pdrid uint16, qfi uint8) error {
	conn := GetConnection()
	session, ok := conn.SmfNodeAssociation.Sessions[localSeid]
	if !ok {
		return fmt.Errorf("failed to find session with localSeid: %d", localSeid)
	}
	pfcpMsg, err := BuildPfcpSessionReportRequest(session.RemoteSEID, getSeqNumber(), pdrid, qfi)
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
	cause, err := rsp.Cause.Cause()
	if err != nil {
		return fmt.Errorf("SMF returned invalid response: %v", err)
	}
	if cause != ie.CauseRequestAccepted {
		return fmt.Errorf("SMF did not accept Session Report Request, cause: %s", causeToString(cause))
	}
	return nil
}
