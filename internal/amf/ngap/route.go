// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Message names for the network event log, matching what getMessageType
// returns for the procedures still on the reference decoder.
const (
	ngSetupRequestMessageType  send.NGAPProcedure = "NGSetupRequest"
	errorIndicationMessageType send.NGAPProcedure = "ErrorIndication"
)

// handleMigrated dispatches the procedures decoded by the in-house NGAP codec
// and reports whether it consumed the message. It returns false for everything
// else, including octets that do not decode at all — the caller's existing path
// reports those. Procedures move here one at a time; when the last one has, this
// becomes the only route and the reference decoder goes.
func handleMigrated(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, span trace.Span) bool {
	pdu, err := ngap.Unmarshal(msg)
	if err != nil {
		return false
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok {
		return false
	}

	switch im.ProcedureCode {
	case ngap.ProcNGSetup:
		receiveNGSetup(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcErrorIndication:
		receiveErrorIndication(ctx, amfInstance, ran, msg, im, span)
	default:
		return false
	}

	return true
}

// traceMessage records what the span and the network event log call a message
// the in-house codec decoded, matching what getMessageType returns for the
// procedures still on the reference decoder.
func traceMessage(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, name send.NGAPProcedure, span trace.Span) {
	span.SetAttributes(
		attribute.String("ngap.message_type", string(name)),
		attribute.Int("ngap.message_size", len(msg)),
		attribute.String("network.protocol.name", "ngap"),
		attribute.String("network.transport", "sctp"),
	)

	amfInstance.LogNetworkEvent(ctx, ran.Conn, name, logger.DirectionInbound, msg)
}

// receiveErrorIndication parses and handles an ERROR INDICATION. A failed parse
// is not answered: TS 38.413 §10.3 forbids replying to an Error Indication with
// another, which would loop.
func receiveErrorIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, errorIndicationMessageType, span)

	ind, err := ngap.ParseErrorIndication(im.Value)
	if err != nil {
		// TS 38.413 §10.3 forbids answering an Error Indication with another,
		// so a failed parse is logged and dropped.
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Error Indication", zap.Error(err))

		return
	}

	HandleErrorIndication(ctx, amfInstance, ran, ind)
}

// receiveNGSetup answers an NG Setup Request using the in-house NGAP codec.
func receiveNGSetup(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	req, parseErr := ngap.ParseNGSetupRequest(im.Value)

	// The peer's RAN node name is applied before the inbound event is logged,
	// so the log keeps chronological order with the outbound response.
	if parseErr == nil && req.RANNodeName != nil && *req.RANNodeName != "" {
		amfInstance.UpdateRadioName(ran, *req.RANNodeName)
	}

	traceMessage(ctx, amfInstance, ran, msg, ngSetupRequestMessageType, span)

	if parseErr != nil {
		span.RecordError(parseErr)
		span.SetStatus(codes.Error, "failed to decode NG Setup Request")

		var ase *ngap.AbstractSyntaxError
		if errors.As(parseErr, &ase) {
			sendNGSetupProtocolFailure(ctx, ran, ase)

			return
		}

		// Octets that decoded as an NG Setup envelope but not as its body. The
		// procedure is known even though no IE is, so the Error Indication
		// cites it (§10.2, §9.3.1.3).
		logger.From(ctx, ran.Log).Error("NG Setup Request decode error", zap.Error(parseErr))
		sendParseErrorIndication(ctx, ran, ngap.ProcNGSetup, parseErr)

		return
	}

	HandleNGSetupRequest(ctx, amfInstance, ran, req)
}
