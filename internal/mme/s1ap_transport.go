// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// S1apPPID is the SCTP payload protocol identifier for S1AP, in host order
// (TS 36.412 §7).
const S1apPPID uint32 = 18

// S1apWirePPID is S1apPPID in the big-endian wire order the socket layer writes
// verbatim (TS 36.412 §7).
var S1apWirePPID = sctp.PPIDWireOrder(S1apPPID)

// SCTP stream identifiers for S1AP signalling: stream 0 is reserved for
// non-UE-associated procedures, and UE-associated signalling uses a distinct,
// stable stream (TS 36.412 §7).
const (
	S1apStreamNonUE uint16 = 0
	S1apStreamUE    uint16 = 1
)

// S1APWriter is the subset of the SCTP connection the MME uses to send S1AP to an
// eNB. *sctp.SCTPConn satisfies it; tests substitute a capturing implementation.
type S1APWriter interface {
	WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error)
}

// SendS1AP writes a complete S1AP PDU to the UE's eNB association.
func (c *UeConn) SendS1AP(ctx context.Context, messageType S1APProcedure, b []byte) {
	if c == nil {
		return
	}

	c.m.SendS1APConn(ctx, c.conn, messageType, b)
}

// SendS1APConn writes a complete S1AP PDU to a specific eNB association, used
// when the target is not the UE's current conn (an in-flight S1 handover) or the
// peer carries no bound UE context.
func (m *MME) SendS1APConn(ctx context.Context, conn S1APWriter, messageType S1APProcedure, b []byte) {
	ctx, span := Tracer.Start(ctx, "s1ap/send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("s1ap.message_type", string(messageType)),
			attribute.Int("s1ap.message_size", len(b)),
			attribute.String("network.protocol.name", "s1ap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: S1apWirePPID, Stream: S1apStreamUE}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send S1AP message")
		logger.From(ctx, logger.MmeLog).Error("failed to send S1AP message", zap.String("message-type", string(messageType)), zap.Error(err))

		return
	}

	m.LogOutboundS1AP(ctx, conn, messageType, b)
}
