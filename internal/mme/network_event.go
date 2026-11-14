// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/sctp"
)

func AddrString(a net.Addr) string {
	if a == nil {
		return ""
	}

	return a.String()
}

func (m *MME) enbIdentityByConn(conn *sctp.SCTPConn, messageType S1APProcedure) (string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.reg.Radio(conn)
	if !ok {
		return "", ""
	}

	if messageType == S1APProcedureS1SetupResponse || messageType == S1APProcedureS1SetupFailure {
		return s.name, s.setupRef
	}

	ref, _ := s.IDKey()

	return s.name, ref
}

func (m *MME) setSetupRef(conn *sctp.SCTPConn, ref string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.reg.Radio(conn); ok {
		s.setupRef = ref
	}
}

// LogNetworkEvent records an S1AP message exchanged with an eNB so it appears in
// the network events log alongside 5G NGAP traffic.
func (m *MME) LogNetworkEvent(ctx context.Context, conn S1APWriter, messageType S1APProcedure, dir logger.LogDirection, raw []byte) {
	m.logNetworkEvent(ctx, conn, messageType, dir, raw, "")
}

func (m *MME) LogSetupRequest(ctx context.Context, conn S1APWriter, messageType S1APProcedure, raw []byte, ref string) {
	if sc, ok := conn.(*sctp.SCTPConn); ok && sc != nil {
		m.setSetupRef(sc, ref)
	}

	m.logNetworkEvent(ctx, conn, messageType, logger.DirectionInbound, raw, ref)
}

func (m *MME) logNetworkEvent(ctx context.Context, conn S1APWriter, messageType S1APProcedure, dir logger.LogDirection, raw []byte, ref string) {
	// Network events carry the SCTP local/remote addresses, so a non-SCTP writer
	// (a test double) has nothing to log.
	sc, ok := conn.(*sctp.SCTPConn)
	if !ok || sc == nil {
		return
	}

	name, connRef := m.enbIdentityByConn(sc, messageType)
	if ref == "" {
		ref = connRef
	}

	logger.LogNetworkEvent(
		ctx,
		logger.S1APNetworkProtocol,
		string(messageType),
		dir,
		AddrString(sc.LocalAddr()),
		AddrString(sc.RemoteAddr()),
		name,
		ref,
		raw,
	)
}

// LogOutboundS1AP records an outbound S1AP message.
func (m *MME) LogOutboundS1AP(ctx context.Context, conn S1APWriter, messageType S1APProcedure, raw []byte) {
	m.LogNetworkEvent(ctx, conn, messageType, logger.DirectionOutbound, raw)
}
