// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ProtectDownlinkMessage serializes a NAS message and integrity-protects and
// ciphers it with the UE's security context.
func (m *MME) ProtectDownlinkMessage(ue *UeContext, msg nasMessage) ([]byte, error) {
	plain, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
}

// SendDownlinkMessage serializes a plain NAS message and sends it to the UE.
func (m *MME) SendDownlinkMessage(ctx context.Context, ue *UeContext, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	m.SendDownlink(ctx, ue, b)
}

// SendDownlinkProtected encodes a plain NAS message, integrity-protects and
// ciphers it with the UE's security context, and sends it downlink.
func (m *MME) SendDownlinkProtected(ctx context.Context, ue *UeContext, msg nasMessage) {
	plain, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	wire, err := ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		logger.MmeLog.Error("failed to protect NAS message", zap.Error(err))
		return
	}

	m.SendDownlink(ctx, ue, wire)
}

// SendDownlink wraps NAS bytes (plain or security-protected) in a Downlink NAS
// Transport and sends them to the UE's eNB.
func (m *MME) SendDownlink(ctx context.Context, ue *UeContext, nas []byte) {
	ctx, span := Tracer.Start(ctx, "s1ap/send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("s1ap.message_type", string(S1APProcedureDownlinkNASTransport)),
			attribute.Int("s1ap.message_size", len(nas)),
			attribute.String("network.protocol.name", "s1ap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	conn, mmeID, enbID := m.S1Identity(ue)
	if conn == nil {
		return
	}

	b, err := downlinkNASTransportBytes(mmeID, enbID, nas)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build Downlink NAS Transport")
		logger.MmeLog.Error("failed to build Downlink NAS Transport", zap.Error(err))

		return
	}

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamUE}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send Downlink NAS Transport")
		logger.MmeLog.Error("failed to send Downlink NAS Transport", zap.Error(err))

		return
	}

	m.LogOutboundS1AP(ctx, conn, S1APProcedureDownlinkNASTransport, b)
}

// SendS1AP writes a complete S1AP PDU to the UE's eNB association.
func (m *MME) SendS1AP(ctx context.Context, ue *UeContext, messageType S1APProcedure, b []byte) {
	conn, _, _ := m.S1Identity(ue)
	if conn == nil {
		return
	}

	m.SendS1APConn(ctx, conn, messageType, b)
}

// SendS1APConn writes a complete S1AP PDU to a specific eNB association, used
// when the target is not the UE's current conn (an in-flight S1 handover).
func (m *MME) SendS1APConn(ctx context.Context, conn NasWriter, messageType S1APProcedure, b []byte) {
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

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamUE}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send S1AP message")
		logger.MmeLog.Error("failed to send S1AP message", zap.String("message-type", string(messageType)), zap.Error(err))

		return
	}

	m.LogOutboundS1AP(ctx, conn, messageType, b)
}
