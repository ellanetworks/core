// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Dispatch decodes an inbound S1AP PDU and routes it to its procedure handler
// (TS 36.413).
func Dispatch(ctx context.Context, m *mme.MME, conn *sctp.SCTPConn, msg []byte) {
	// Inbound S1AP carries no propagated trace context; fresh root span.
	ctx, span := mme.Tracer.Start(ctx, "s1ap/receive",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.Int("s1ap.message_size", len(msg)),
			attribute.String("network.protocol.name", "s1ap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	if conn != nil {
		span.SetAttributes(
			attribute.String("network.peer.address", mme.AddrString(conn.RemoteAddr())),
			attribute.String("network.local.address", mme.AddrString(conn.LocalAddr())),
		)
	}

	pdu, err := s1ap.Unmarshal(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode S1AP PDU")
		logger.From(ctx, m.RadioLog(conn)).Warn("failed to decode S1AP PDU", zap.Error(err))

		return
	}

	messageType := mme.S1APMessageType(pdu)
	span.SetAttributes(attribute.String("s1ap.message_type", string(messageType)))

	// Track the eNB before logging so the inbound event is attributed to the radio.
	isSetup := false
	if im, ok := pdu.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcS1Setup {
		isSetup = true

		m.TrackRadioFromSetup(conn, im.Value)
	}

	// Resolve the eNB once at ingress and thread it to handlers; nil before S1 Setup
	// records the eNB.
	radio := m.RadioForConn(conn)
	if radio != nil {
		radio.TouchLastSeen()
	}

	m.LogNetworkEvent(ctx, conn, messageType, logger.DirectionInbound, msg)

	// TS 36.413: S1 Setup is the first procedure on a TNL association. Until it
	// completes, drop every other message, including UE signalling from an eNB
	// whose S1 Setup was rejected.
	if !isSetup && (radio == nil || !radio.SetupComplete()) {
		logger.From(ctx, m.RadioLog(conn)).Warn("S1AP message before S1 Setup, dropping",
			zap.String("message-type", string(messageType)))

		return
	}

	if im, ok := pdu.(*s1ap.InitiatingMessage); ok {
		switch im.ProcedureCode {
		case s1ap.ProcS1Setup:
			// The radio-creating handler: radio may still be nil on a parse failure,
			// so it keeps the raw conn.
			handleS1Setup(m, ctx, conn, im.Value)

			return
		case s1ap.ProcENBConfigurationUpdate:
			handleENBConfigurationUpdate(m, ctx, radio, im.Value)

			return
		}
	}

	Route(m, ctx, radio, pdu)
}
