// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var ngapSendTracer = otel.Tracer("ella-core/amf/ngap/send")

// NGAPWriter is the subset of the SCTP connection the AMF uses to send NGAP to a
// gNB. *sctp.SCTPConn satisfies it; tests substitute a capturing implementation.
type NGAPWriter interface {
	WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error)
}

// SendToRadio writes a complete NGAP PDU to this radio's gNB association. This is the
// node-level, fire-and-forget path: a send failure is logged at the chokepoint and not
// returned. Callers that must act on the send outcome (session/ICS setup) use
// (*AMF).SendToRadio directly.
func (r *Radio) SendToRadio(ctx context.Context, msgType send.NGAPProcedure, packet []byte) {
	if r == nil || r.amf == nil {
		logger.From(ctx, logger.AmfLog).Error("cannot send NGAP message: radio is not bound to an amf", zap.String("message_type", string(msgType)))
		return
	}

	// The send failure, if any, is logged at the chokepoint.
	_ = r.amf.SendToRadio(ctx, r.Conn, msgType, packet)
}

// SendDownlinkNRPPaTransport builds a DOWNLINK UE-ASSOCIATED NRPPa TRANSPORT carrying
// the LMF's NRPPa PDU and sends it to this radio's gNB (TS 38.413 §8.14.2). It returns
// the send outcome so the LMF positioning client can report a delivery failure.
func (r *Radio) SendDownlinkNRPPaTransport(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, routingID int64, nrppaPdu []byte) error {
	if r == nil || r.amf == nil {
		return fmt.Errorf("radio is not bound to an amf")
	}

	pkt, err := send.BuildDownlinkUEAssociatedNRPPaTransport(amfUeNgapID, ranUeNgapID, routingID, nrppaPdu)
	if err != nil {
		return fmt.Errorf("build downlink NRPPa transport: %w", err)
	}

	return r.amf.SendToRadio(ctx, r.Conn, send.NGAPProcedureDownlinkNRPPaTransport, pkt)
}

// SendToRadio writes a complete NGAP PDU to a gNB association, selecting the SCTP
// stream from the procedure (TS 38.412). It takes the connection (the send target)
// directly — a UE sends through ueConn.conn.
func (a *AMF) SendToRadio(ctx context.Context, conn NGAPWriter, msgType send.NGAPProcedure, packet []byte) error {
	ctx, span := ngapSendTracer.Start(ctx, "ngap/send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("ngap.message_type", string(msgType)),
			attribute.Int("ngap.message_size", len(packet)),
			attribute.String("network.protocol.name", "ngap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	if conn == nil {
		return fmt.Errorf("ran conn is nil")
	}

	if len(packet) == 0 {
		return fmt.Errorf("packet len is 0")
	}

	sid, err := send.GetSCTPStreamID(msgType)
	if err != nil {
		return fmt.Errorf("could not determine SCTP stream ID from NGAP message type (%s): %w", msgType, err)
	}

	info := sctp.SndRcvInfo{
		Stream: sid,
		PPID:   sctp.PPIDWireOrder(send.NGAPPPID),
	}
	if _, err := conn.WriteMsg(packet, &info); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send NGAP message")
		logger.From(ctx, logger.AmfLog).Error("failed to send NGAP message", zap.String("message_type", string(msgType)), zap.Error(err))

		return fmt.Errorf("send write to sctp connection: %w", err)
	}

	a.logOutboundNGAP(ctx, conn, msgType, packet)

	return nil
}
