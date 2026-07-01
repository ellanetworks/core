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
	"go.opentelemetry.io/otel/trace"
)

var ngapSendTracer = otel.Tracer("ella-core/amf/ngap/send")

// NGAPWriter is the subset of the SCTP connection the AMF uses to send NGAP to a
// gNB. *sctp.SCTPConn satisfies it; tests substitute a capturing implementation.
type NGAPWriter interface {
	WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error)
}

// SendToRan writes a complete NGAP PDU to this radio's gNB association.
func (r *Radio) SendToRan(ctx context.Context, msgType send.NGAPProcedure, packet []byte) error {
	if r == nil || r.amf == nil {
		return fmt.Errorf("radio is not bound to an amf")
	}

	return r.amf.SendToRan(ctx, r, msgType, packet)
}

// SendToRan writes a complete NGAP PDU to a radio's gNB association, selecting
// the SCTP stream from the procedure (TS 38.412).
func (a *AMF) SendToRan(ctx context.Context, radio *Radio, msgType send.NGAPProcedure, packet []byte) error {
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

	if radio == nil || radio.Conn == nil {
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
	if _, err := radio.Conn.WriteMsg(packet, &info); err != nil {
		return fmt.Errorf("send write to sctp connection: %w", err)
	}

	a.logOutboundNGAP(ctx, radio, msgType, packet)

	return nil
}

// logOutboundNGAP records a sent NGAP PDU as a network event. Addresses come
// from the concrete SCTP connection; a test writer is skipped.
func (a *AMF) logOutboundNGAP(ctx context.Context, radio *Radio, msgType send.NGAPProcedure, packet []byte) {
	conn, ok := radio.Conn.(*sctp.SCTPConn)
	if !ok {
		return
	}

	localAddr := conn.LocalAddr()
	remoteAddr := conn.RemoteAddr()

	if localAddr == nil || remoteAddr == nil {
		return
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		string(msgType),
		logger.DirectionOutbound,
		localAddr.String(),
		remoteAddr.String(),
		radio.Name,
		packet,
	)
}
