// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/radioctx"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/core/sctp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Dispatch decodes an inbound S1AP PDU and routes it to its procedure handler
// (TS 36.413).
func Dispatch(ctx context.Context, m *mme.MME, conn *sctp.SCTPConn, msg []byte) {
	pdu, err := s1ap.Unmarshal(msg)
	if err == nil && conn != nil {
		ctx = ueTraceContext(ctx, m, conn, pdu)
	}

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

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode S1AP PDU")
		m.RadioLog(ctx, conn).Warn("failed to decode S1AP PDU", zap.Error(err))

		if conn != nil {
			sendProtocolErrorIndication(ctx, m, conn, s1ap.CauseProtocolTransferSyntaxError)
		}

		return
	}

	messageType := mme.S1APMessageType(pdu)
	span.SetAttributes(attribute.String("s1ap.message_type", string(messageType)))

	// Track the eNB before logging so the inbound event is attributed to the radio.
	isSetup := false
	setupRef := ""

	if im, ok := pdu.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcS1Setup {
		isSetup = true

		setupRef = m.TrackRadioFromSetup(ctx, conn, im.Value)
	}

	// radio is nil until S1 Setup records the eNB.
	radio := m.RadioForConn(conn)
	if radio != nil {
		radio.TouchLastSeen()

		if id, ok := radio.IDKey(); ok {
			ctx = radioctx.With(ctx, id)
		}
	}

	if isSetup {
		m.LogSetupRequest(ctx, conn, messageType, msg, setupRef)
	} else {
		m.LogNetworkEvent(ctx, conn, messageType, logger.DirectionInbound, msg)
	}

	// TS 36.413: S1 Setup is the first procedure on a TNL association. Until it
	// completes, drop every other message, including UE signalling from an eNB
	// whose S1 Setup was rejected.
	if !isSetup && (radio == nil || !radio.SetupComplete()) {
		m.RadioLog(ctx, conn).Warn("S1AP message before S1 Setup, dropping",
			logger.MessageType(string(messageType)))

		return
	}

	// S1 Setup creates the radio, so it takes the raw conn (radio may be nil on a parse
	// failure); every other procedure is dispatched by Route (TS 36.413).
	if im, ok := pdu.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcS1Setup {
		handleS1Setup(ctx, m, conn, im.Value)

		return
	}

	Route(ctx, m, radio, pdu)
}

func ueTraceContext(ctx context.Context, m *mme.MME, conn *sctp.SCTPConn, pdu s1ap.PDU) context.Context {
	id, ok := s1ap.PeekMMEUES1APID(pdu)
	if !ok {
		return ctx
	}

	im, initiating := pdu.(*s1ap.InitiatingMessage)
	anyRadio := initiating && im.ProcedureCode == s1ap.ProcPathSwitchRequest

	return m.UETraceContext(ctx, conn, id, anyRadio)
}
