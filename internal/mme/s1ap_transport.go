// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/sctp"
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
	WriteMsgWait(ctx context.Context, b []byte, info *sctp.SndRcvInfo) (int, error)
}

type radioSend func(m *MME, ctx context.Context, conn S1APWriter, messageType S1APProcedure, b []byte) error

// SendS1AP writes a complete S1AP PDU to the UE's eNB association.
func (c *UeConn) SendS1AP(ctx context.Context, messageType S1APProcedure, b []byte) error {
	return c.sendS1AP(ctx, messageType, b, (*MME).SendToRadio)
}

func (c *UeConn) sendS1AP(ctx context.Context, messageType S1APProcedure, b []byte, send radioSend) error {
	if c == nil {
		return fmt.Errorf("no S1 connection to send %s on", messageType)
	}

	return send(c.m, ctx, c.Conn(), messageType, b)
}

// s1apStreamForProcedure returns the SCTP stream for an S1AP procedure: the reserved
// non-UE stream (0) for non-UE-associated signalling, the UE stream otherwise
// (TS 36.412 §7).
func s1apStreamForProcedure(p S1APProcedure) (uint16, error) {
	switch p {
	case S1APProcedureS1SetupResponse, S1APProcedureS1SetupFailure,
		S1APProcedurePaging, S1APProcedureResetAcknowledge,
		S1APProcedureErrorIndication,
		S1APProcedureENBConfigUpdateAck, S1APProcedureENBConfigUpdateFailure,
		S1APProcedureMMEConfigUpdate,
		S1APProcedureMMEConfigurationTransfer:
		return S1apStreamNonUE, nil
	case S1APProcedureInitialContextSetupRequest, S1APProcedureUEContextReleaseCommand,
		S1APProcedureUEContextModRequest,
		S1APProcedureDownlinkNASTransport,
		S1APProcedureERABSetupRequest, S1APProcedureERABModifyRequest, S1APProcedureERABReleaseCommand,
		S1APProcedureERABModificationConfirm,
		S1APProcedureHandoverRequest, S1APProcedureHandoverCommand, S1APProcedureHandoverPreparationFailure,
		S1APProcedureHandoverCancelAcknowledge, S1APProcedureMMEStatusTransfer,
		S1APProcedurePathSwitchRequestAck, S1APProcedurePathSwitchRequestFailure,
		S1APProcedureDownlinkUEAssociatedLPPaTransport:
		return S1apStreamUE, nil
	default:
		return 0, fmt.Errorf("no SCTP stream for S1AP message type %s", p)
	}
}

// SendToRadio writes a complete S1AP PDU to a specific eNB association — the single
// traced+logged send chokepoint. The SCTP stream is derived from the procedure.
func (m *MME) SendToRadio(ctx context.Context, conn S1APWriter, messageType S1APProcedure, b []byte) error {
	return m.sendToRadio(ctx, conn, messageType, b, false)
}

func (m *MME) SendToRadioPaced(ctx context.Context, conn S1APWriter, messageType S1APProcedure, b []byte) error {
	return m.sendToRadio(ctx, conn, messageType, b, true)
}

func (m *MME) sendToRadio(ctx context.Context, conn S1APWriter, messageType S1APProcedure, b []byte, paced bool) error {
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

	if conn == nil {
		logger.From(ctx, logger.MmeLog).Error("cannot send S1AP message: eNB connection is nil", logger.MessageType(string(messageType)))
		return fmt.Errorf("no eNB association to send %s on", messageType)
	}

	stream, err := s1apStreamForProcedure(messageType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send S1AP message")
		logger.From(ctx, logger.MmeLog).Error("cannot send S1AP message", logger.MessageType(string(messageType)), zap.Error(err))

		return err
	}

	info := sctp.SndRcvInfo{PPID: S1apWirePPID, Stream: stream}

	if paced {
		_, err = conn.WriteMsgWait(ctx, b, &info)
	} else {
		_, err = conn.WriteMsg(b, &info)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send S1AP message")
		logger.From(ctx, logger.MmeLog).Error("failed to send S1AP message", logger.MessageType(string(messageType)), zap.Error(err))

		return fmt.Errorf("send %s: %w", messageType, err)
	}

	m.LogOutboundS1AP(ctx, conn, messageType, b)

	return nil
}
