// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// downlinkNASTransportBytes builds a Downlink NAS Transport PDU carrying nas for
// the given S1AP identities (TS 36.413).
func downlinkNASTransportBytes(mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID, nas []byte) ([]byte, error) {
	msg := &s1ap.DownlinkNASTransport{
		MMEUES1APID: mmeID,
		ENBUES1APID: enbID,
		NASPDU:      s1ap.NASPDU(nas),
	}

	return msg.Marshal()
}

// nasMessage is any EPS NAS message that can serialize itself.
type nasMessage interface {
	Marshal() ([]byte, error)
}

// SendOverConn wraps a plain NAS message in a Downlink NAS Transport and sends it
// over a connection that carries no bound UE context — a reject to an
// unidentified peer (an Initial UE Message the MME cannot act on).
func (m *MME) SendOverConn(ctx context.Context, c *S1Conn, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	pdu, err := downlinkNASTransportBytes(c.MMEUES1APID, c.ENBUES1APID, b)
	if err != nil {
		logger.MmeLog.Error("failed to build Downlink NAS Transport", zap.Error(err))
		return
	}

	if _, err := c.conn.WriteMsg(pdu, &sctp.SndRcvInfo{PPID: S1apWirePPID, Stream: S1apStreamUE}); err != nil {
		logger.MmeLog.Error("failed to send Downlink NAS Transport", zap.Error(err))
		return
	}

	m.LogOutboundS1AP(ctx, c.conn, S1APProcedureDownlinkNASTransport, pdu)
}
