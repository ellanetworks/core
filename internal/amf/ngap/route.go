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
	ngResetMessageType         send.NGAPProcedure = "NGReset"

	ranConfigurationUpdateMessageType send.NGAPProcedure = "RANConfigurationUpdate"

	nasNonDeliveryIndicationMessageType send.NGAPProcedure = "NASNonDeliveryIndication"
	uplinkNASTransportMessageType       send.NGAPProcedure = "UplinkNASTransport"
	initialUEMessageMessageType         send.NGAPProcedure = "InitialUEMessage"

	uplinkRANConfigurationTransferMessageType send.NGAPProcedure = "UplinkRANConfigurationTransfer"
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

	// TS 38.413 §8.7.1.1: "This procedure shall be the first NGAP procedure
	// triggered after the TNL association has become operational." The reference
	// decoder's path enforces that further down the dispatcher, but this one
	// returns before reaching it, so it has to gate itself — otherwise a peer
	// that never completed NG Setup could reach a handler, and RAN Configuration
	// Update would let it claim a Global RAN Node ID and open the gate for
	// everything else. The MME gates the same way (its dispatcher drops
	// everything but S1 Setup until SetupComplete).
	if im.ProcedureCode != ngap.ProcNGSetup && ran.RanID == nil {
		logger.From(ctx, ran.Log).Warn("NGAP message before NG Setup, dropping",
			zap.String("procedure", im.ProcedureCode.String()))

		return true
	}

	switch im.ProcedureCode {
	case ngap.ProcNGSetup:
		receiveNGSetup(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcErrorIndication:
		receiveErrorIndication(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcNGReset:
		receiveNGReset(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcRANConfigurationUpdate:
		receiveRANConfigurationUpdate(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcUplinkRANConfigurationTransfer:
		receiveUplinkRANConfigurationTransfer(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcNASNonDeliveryIndication:
		receiveNASNonDeliveryIndication(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcUplinkNASTransport:
		receiveUplinkNASTransport(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcInitialUEMessage:
		receiveInitialUEMessage(ctx, amfInstance, ran, msg, im, span)
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
// is not answered: TS 38.413 §10.5 forbids replying to an Error Indication with
// another, which would loop.
func receiveErrorIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, errorIndicationMessageType, span)

	ind, err := ngap.ParseErrorIndication(im.Value)
	if err != nil {
		// TS 38.413 §10.5 forbids answering an Error Indication with another,
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

// receiveNGReset parses and handles an NG RESET. A failed parse is answered
// with an Error Indication: NG Reset defines no unsuccessful outcome
// (TS 38.413 §10.3.5).
func receiveNGReset(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, ngResetMessageType, span)

	req, err := ngap.ParseNGReset(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode NG Reset", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcNGReset, err)

		return
	}

	HandleNGReset(ctx, amfInstance, ran, req)
}

// receiveRANConfigurationUpdate parses and handles a RAN CONFIGURATION UPDATE.
// The procedure defines an unsuccessful outcome, so a failed parse is answered
// there rather than with an Error Indication (TS 38.413 §10.3.5).
func receiveRANConfigurationUpdate(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, ranConfigurationUpdateMessageType, span)

	req, err := ngap.ParseRANConfigurationUpdate(im.Value)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode RAN Configuration Update")

		var ase *ngap.AbstractSyntaxError
		if errors.As(err, &ase) {
			sendRANConfigurationUpdateProtocolFailure(ctx, ran, ase)

			return
		}

		// Octets that decoded as the envelope but not as its body. The
		// procedure is known even though no IE is, so the Error Indication
		// cites it (§10.2, §9.3.1.3).
		logger.From(ctx, ran.Log).Error("RAN Configuration Update decode error", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcRANConfigurationUpdate, err)

		return
	}

	HandleRANConfigurationUpdate(ctx, amfInstance, ran, req)
}

// receiveUplinkRANConfigurationTransfer parses and handles an UPLINK RAN
// CONFIGURATION TRANSFER. The procedure defines no unsuccessful outcome, so a
// failed parse is answered with an Error Indication (TS 38.413 §10.3.5).
func receiveUplinkRANConfigurationTransfer(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, uplinkRANConfigurationTransferMessageType, span)

	req, err := ngap.ParseUplinkRANConfigurationTransfer(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Uplink RAN Configuration Transfer", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcUplinkRANConfigurationTransfer, err)

		return
	}

	HandleUplinkRANConfigurationTransfer(ctx, amfInstance, ran, req)
}

// receiveNASNonDeliveryIndication parses and handles a NAS NON DELIVERY
// INDICATION. Both UE NGAP IDs are mandatory with reject criticality, so a
// message missing either does not reach the handler.
func receiveNASNonDeliveryIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, nasNonDeliveryIndicationMessageType, span)

	ind, err := ngap.ParseNASNonDeliveryIndication(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode NAS Non Delivery Indication", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcNASNonDeliveryIndication, err)

		return
	}

	HandleNASNonDeliveryIndication(ctx, amfInstance, ran, ind)
}

// receiveUplinkNASTransport parses and handles an UPLINK NAS TRANSPORT. The
// procedure defines no unsuccessful outcome, so a failed parse is answered with
// an Error Indication (TS 38.413 §10.3.5).
func receiveUplinkNASTransport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, uplinkNASTransportMessageType, span)

	req, err := ngap.ParseUplinkNASTransport(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Uplink NAS Transport", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcUplinkNASTransport, err)

		return
	}

	HandleUplinkNASTransport(ctx, amfInstance, ran, req)
}

// receiveInitialUEMessage parses and handles an INITIAL UE MESSAGE. The
// procedure defines no unsuccessful outcome, so a failed parse is answered with
// an Error Indication (TS 38.413 §10.3.5).
func receiveInitialUEMessage(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, initialUEMessageMessageType, span)

	req, err := ngap.ParseInitialUEMessage(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Initial UE Message", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcInitialUEMessage, err)

		return
	}

	HandleInitialUEMessage(ctx, amfInstance, ran, req)
}
