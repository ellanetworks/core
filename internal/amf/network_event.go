// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/sctp"
)

// AddrString renders an SCTP address for logging, returning "" for a nil address (a
// closed/half-established association) so the record is still logged.
func AddrString(a net.Addr) string {
	if a == nil {
		return ""
	}

	return a.String()
}

// LogNetworkEvent records an NGAP message exchanged with a gNB in the network events
// log. Addresses come from the radio's concrete SCTP connection; a test writer (or a
// nil connection) is skipped. A nil address renders as "" so the exchange is still logged.
func (a *AMF) LogNetworkEvent(ctx context.Context, conn NGAPWriter, messageType NGAPProcedure, dir logger.LogDirection, raw []byte) {
	a.logNetworkEvent(ctx, conn, messageType, dir, raw, "")
}

func (a *AMF) LogSetupRequest(ctx context.Context, conn NGAPWriter, messageType NGAPProcedure, raw []byte, ref string) {
	if conn != nil {
		a.setSetupRef(conn, ref)
	}

	a.logNetworkEvent(ctx, conn, messageType, logger.DirectionInbound, raw, ref)
}

func (a *AMF) logNetworkEvent(ctx context.Context, conn NGAPWriter, messageType NGAPProcedure, dir logger.LogDirection, raw []byte, ref string) {
	if conn == nil {
		return
	}

	sctpConn, ok := conn.(*sctp.SCTPConn)
	if !ok {
		return
	}

	name, connRef := a.radioIdentityByConn(conn, messageType)
	if ref == "" {
		ref = connRef
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		string(messageType),
		dir,
		AddrString(sctpConn.LocalAddr()),
		AddrString(sctpConn.RemoteAddr()),
		name,
		ref,
		raw,
	)
}

// logOutboundNGAP records a sent NGAP PDU as a network event.
func (a *AMF) logOutboundNGAP(ctx context.Context, conn NGAPWriter, msgType NGAPProcedure, packet []byte) {
	a.LogNetworkEvent(ctx, conn, msgType, logger.DirectionOutbound, packet)
}
