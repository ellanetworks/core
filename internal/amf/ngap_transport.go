// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/sctp"
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
	WriteMsgWait(ctx context.Context, b []byte, info *sctp.SndRcvInfo) (int, error)
}

type radioSend func(a *AMF, ctx context.Context, conn NGAPWriter, msgType NGAPProcedure, packet []byte) error

// SendToRadio writes a complete NGAP PDU to this radio's gNB association. This is the
// node-level, fire-and-forget path: a send failure is logged at the chokepoint and not
// returned. Callers that must act on the send outcome (session/ICS setup) use
// (*AMF).SendToRadio directly.
func (r *Radio) SendToRadio(ctx context.Context, msgType NGAPProcedure, packet []byte) {
	if r == nil || r.amf == nil {
		logger.From(ctx, logger.AmfLog).Error("cannot send NGAP message: radio is not bound to an amf", logger.MessageType(string(msgType)))
		return
	}

	// The send failure, if any, is logged at the chokepoint.
	_ = r.amf.SendToRadio(ctx, r.Conn, msgType, packet)
}

// SendToRadio writes a complete NGAP PDU to a gNB association, selecting the SCTP
// stream from the procedure (TS 38.412). It takes the connection (the send target)
// directly — a UE sends through ueConn.conn.
func (a *AMF) SendToRadio(ctx context.Context, conn NGAPWriter, msgType NGAPProcedure, packet []byte) error {
	return a.sendToRadio(ctx, conn, msgType, packet, false)
}

func (a *AMF) SendToRadioPaced(ctx context.Context, conn NGAPWriter, msgType NGAPProcedure, packet []byte) error {
	return a.sendToRadio(ctx, conn, msgType, packet, true)
}

func (a *AMF) sendToRadio(ctx context.Context, conn NGAPWriter, msgType NGAPProcedure, packet []byte, paced bool) error {
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

	sid, err := GetSCTPStreamID(msgType)
	if err != nil {
		return fmt.Errorf("could not determine SCTP stream ID from NGAP message type (%s): %w", msgType, err)
	}

	info := sctp.SndRcvInfo{
		Stream: sid,
		PPID:   sctp.PPIDWireOrder(NGAPPPID),
	}

	if paced {
		_, err = conn.WriteMsgWait(ctx, packet, &info)
	} else {
		_, err = conn.WriteMsg(packet, &info)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send NGAP message")
		logger.From(ctx, logger.AmfLog).Error("failed to send NGAP message", logger.MessageType(string(msgType)), zap.Error(err))

		return fmt.Errorf("send write to sctp connection: %w", err)
	}

	a.logOutboundNGAP(ctx, conn, msgType, packet)

	return nil
}
