// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
)

func AddrString(a net.Addr) string {
	if a == nil {
		return ""
	}

	return a.String()
}

func (m *MME) enbNameByConn(conn *sctp.SCTPConn) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if s := m.enbs[conn]; s != nil {
		return s.name
	}

	return ""
}

// LogNetworkEvent records an S1AP message exchanged with an eNB so it appears in
// the network events log alongside 5G NGAP traffic.
func (m *MME) LogNetworkEvent(ctx context.Context, conn *sctp.SCTPConn, messageType S1APProcedure, dir logger.LogDirection, raw []byte) {
	if conn == nil {
		return
	}

	logger.LogNetworkEvent(
		ctx,
		logger.S1APNetworkProtocol,
		string(messageType),
		dir,
		AddrString(conn.LocalAddr()),
		AddrString(conn.RemoteAddr()),
		m.enbNameByConn(conn),
		raw,
	)
}

// LogOutboundS1AP records an outbound S1AP message. The UE-facing writer is the
// SCTP association; events from non-SCTP writers (tests) are skipped.
func (m *MME) LogOutboundS1AP(ctx context.Context, conn NasWriter, messageType S1APProcedure, raw []byte) {
	sctpConn, ok := conn.(*sctp.SCTPConn)
	if !ok {
		return
	}

	m.LogNetworkEvent(ctx, sctpConn, messageType, logger.DirectionOutbound, raw)
}
