// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
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

	return r.amf.SendToRan(ctx, r.Conn, msgType, packet)
}

// SendDownlinkNRPPaTransport builds a DOWNLINK UE-ASSOCIATED NRPPa TRANSPORT carrying
// the LMF's NRPPa PDU and sends it to this radio's gNB (TS 38.413 §8.14.2). The LMF
// positioning client uses this to reach the RAN through the AMF's NGAP association.
func (r *Radio) SendDownlinkNRPPaTransport(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, routingID int64, nrppaPdu []byte) error {
	pkt, err := send.BuildDownlinkUEAssociatedNRPPaTransport(amfUeNgapID, ranUeNgapID, routingID, nrppaPdu)
	if err != nil {
		return fmt.Errorf("build downlink NRPPa transport: %w", err)
	}

	return r.SendToRan(ctx, send.NGAPProcedureDownlinkNRPPaTransport, pkt)
}

// SendToRan writes a complete NGAP PDU to a gNB association, selecting the SCTP
// stream from the procedure (TS 38.412). It takes the connection (the send target)
// directly — a UE sends through ueConn.conn — mirroring the MME's SendS1APConn.
func (a *AMF) SendToRan(ctx context.Context, conn NGAPWriter, msgType send.NGAPProcedure, packet []byte) error {
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
		return fmt.Errorf("send write to sctp connection: %w", err)
	}

	a.logOutboundNGAP(ctx, conn, msgType, packet)

	return nil
}
