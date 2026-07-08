// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
)

// LogNetworkEvent records an NGAP message exchanged with a gNB so it appears in
// the network events log alongside 4G S1AP traffic (mirrors the MME's
// LogNetworkEvent). Addresses come from the radio's concrete SCTP connection; a
// test writer (or a nil radio/connection) is skipped.
func (a *AMF) LogNetworkEvent(ctx context.Context, conn NGAPWriter, messageType string, dir logger.LogDirection, raw []byte) {
	if conn == nil {
		return
	}

	sctpConn, ok := conn.(*sctp.SCTPConn)
	if !ok {
		return
	}

	localAddr := sctpConn.LocalAddr()
	remoteAddr := sctpConn.RemoteAddr()

	if localAddr == nil || remoteAddr == nil {
		return
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		messageType,
		dir,
		localAddr.String(),
		remoteAddr.String(),
		a.radioNameByConn(conn),
		raw,
	)
}

// logOutboundNGAP records a sent NGAP PDU as a network event.
func (a *AMF) logOutboundNGAP(ctx context.Context, conn NGAPWriter, msgType send.NGAPProcedure, packet []byte) {
	a.LogNetworkEvent(ctx, conn, string(msgType), logger.DirectionOutbound, packet)
}
