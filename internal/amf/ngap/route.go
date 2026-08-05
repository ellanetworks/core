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

	nasNonDeliveryIndicationMessageType           send.NGAPProcedure = "NASNonDeliveryIndication"
	uplinkNASTransportMessageType                 send.NGAPProcedure = "UplinkNASTransport"
	initialUEMessageMessageType                   send.NGAPProcedure = "InitialUEMessage"
	ueContextReleaseRequestMessageType            send.NGAPProcedure = "UEContextReleaseRequest"
	ueContextReleaseCompleteMessageType           send.NGAPProcedure = "UEContextReleaseComplete"
	initialContextSetupResponseMessageType        send.NGAPProcedure = "InitialContextSetupResponse"
	initialContextSetupFailureMessageType         send.NGAPProcedure = "InitialContextSetupFailure"
	ueRadioCapabilityInfoIndicationMessageType    send.NGAPProcedure = "UERadioCapabilityInfoIndication"
	pduSessionResourceSetupResponseMessageType    send.NGAPProcedure = "PDUSessionResourceSetupResponse"
	pduSessionResourceReleaseResponseMessageType  send.NGAPProcedure = "PDUSessionResourceReleaseResponse"
	pduSessionResourceModifyResponseMessageType   send.NGAPProcedure = "PDUSessionResourceModifyResponse"
	pduSessionResourceModifyIndicationMessageType send.NGAPProcedure = "PDUSessionResourceModifyIndication"
	pduSessionResourceNotifyMessageType           send.NGAPProcedure = "PDUSessionResourceNotify"
	handoverNotifyMessageType                     send.NGAPProcedure = "HandoverNotify"
	handoverCancelMessageType                     send.NGAPProcedure = "HandoverCancel"
	handoverRequiredMessageType                   send.NGAPProcedure = "HandoverRequired"
	handoverRequestAcknowledgeMessageType         send.NGAPProcedure = "HandoverRequestAcknowledge"
	handoverFailureMessageType                    send.NGAPProcedure = "HandoverFailure"
	uplinkRANStatusTransferMessageType            send.NGAPProcedure = "UplinkRANStatusTransfer"

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

	switch p := pdu.(type) {
	case *ngap.InitiatingMessage:
		return routeInitiating(ctx, amfInstance, ran, msg, p, span)
	case *ngap.SuccessfulOutcome:
		return routeSuccessful(ctx, amfInstance, ran, msg, p, span)
	case *ngap.UnsuccessfulOutcome:
		return routeUnsuccessful(ctx, amfInstance, ran, msg, p, span)
	default:
		return false
	}
}

func routeInitiating(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) bool {
	// TS 38.413 §8.7.1.1: "This procedure shall be the first NGAP procedure
	// triggered after the TNL association has become operational." The reference
	// decoder's path enforces that further down the dispatcher, but this one
	// returns before reaching it, so it has to gate itself — otherwise a peer
	// that never completed NG Setup could reach a handler, and RAN Configuration
	// Update would let it claim a Global RAN Node ID and open the gate for
	// everything else. The MME gates the same way (its dispatcher drops
	// everything but S1 Setup until SetupComplete).
	if !setupComplete(ctx, ran, im.ProcedureCode) {
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
	case ngap.ProcUEContextReleaseRequest:
		receiveUEContextReleaseRequest(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcUERadioCapabilityInfoIndication:
		receiveUERadioCapabilityInfoIndication(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcPDUSessionResourceModifyIndication:
		receivePDUSessionResourceModifyIndication(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcPDUSessionResourceNotify:
		receivePDUSessionResourceNotify(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcHandoverNotification:
		receiveHandoverNotify(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcHandoverCancel:
		receiveHandoverCancel(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcHandoverPreparation:
		receiveHandoverRequired(ctx, amfInstance, ran, msg, im, span)
	case ngap.ProcUplinkRANStatusTransfer:
		receiveUplinkRANStatusTransfer(ctx, amfInstance, ran, msg, im, span)
	default:
		return false
	}

	return true
}

func routeSuccessful(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, so *ngap.SuccessfulOutcome, span trace.Span) bool {
	if !setupComplete(ctx, ran, so.ProcedureCode) {
		return true
	}

	switch so.ProcedureCode {
	case ngap.ProcHandoverResourceAllocation:
		receiveHandoverRequestAcknowledge(ctx, amfInstance, ran, msg, so, span)
	case ngap.ProcUEContextRelease:
		receiveUEContextReleaseComplete(ctx, amfInstance, ran, msg, so, span)
	case ngap.ProcInitialContextSetup:
		receiveInitialContextSetupResponse(ctx, amfInstance, ran, msg, so, span)
	case ngap.ProcPDUSessionResourceSetup:
		receivePDUSessionResourceSetupResponse(ctx, amfInstance, ran, msg, so, span)
	case ngap.ProcPDUSessionResourceRelease:
		receivePDUSessionResourceReleaseResponse(ctx, amfInstance, ran, msg, so, span)
	case ngap.ProcPDUSessionResourceModify:
		receivePDUSessionResourceModifyResponse(ctx, amfInstance, ran, msg, so, span)
	default:
		return false
	}

	return true
}

func routeUnsuccessful(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, uo *ngap.UnsuccessfulOutcome, span trace.Span) bool {
	if !setupComplete(ctx, ran, uo.ProcedureCode) {
		return true
	}

	switch uo.ProcedureCode {
	case ngap.ProcHandoverResourceAllocation:
		receiveHandoverFailure(ctx, amfInstance, ran, msg, uo, span)
	case ngap.ProcInitialContextSetup:
		receiveInitialContextSetupFailure(ctx, amfInstance, ran, msg, uo, span)
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

// setupComplete reports whether proc may be handled on this association.
func setupComplete(ctx context.Context, ran *amf.Radio, proc ngap.ProcedureCode) bool {
	if proc == ngap.ProcNGSetup || ran.RanID != nil {
		return true
	}

	logger.From(ctx, ran.Log).Warn("NGAP message before NG Setup, dropping",
		zap.String("procedure", proc.String()))

	return false
}

// receiveUEContextReleaseRequest parses and handles a UE CONTEXT RELEASE
// REQUEST. The procedure defines no unsuccessful outcome, so a failed parse is
// answered with an Error Indication (TS 38.413 §10.3.5).
func receiveUEContextReleaseRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, ueContextReleaseRequestMessageType, span)

	req, err := ngap.ParseUEContextReleaseRequest(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode UE Context Release Request", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcUEContextReleaseRequest, err)

		return
	}

	HandleUEContextReleaseRequest(ctx, amfInstance, ran, req)
}

// receiveUEContextReleaseComplete parses and handles a UE CONTEXT RELEASE
// COMPLETE. It is the successful outcome of a procedure the AMF itself started,
// so a failed parse is answered with an Error Indication (TS 38.413 §10.3.5).
func receiveUEContextReleaseComplete(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, so *ngap.SuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, ueContextReleaseCompleteMessageType, span)

	cpl, err := ngap.ParseUEContextReleaseComplete(so.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode UE Context Release Complete", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcUEContextRelease, err)

		return
	}

	HandleUEContextReleaseComplete(ctx, amfInstance, ran, cpl)
}

// receiveInitialContextSetupResponse parses and handles an INITIAL CONTEXT
// SETUP RESPONSE. A failed parse is answered with an Error Indication: the
// unsuccessful outcome reports the NG-RAN node's failure, not the AMF's
// (TS 38.413 §10.3.5).
func receiveInitialContextSetupResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, so *ngap.SuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, initialContextSetupResponseMessageType, span)

	resp, err := ngap.ParseInitialContextSetupResponse(so.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Initial Context Setup Response", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcInitialContextSetup, err)

		return
	}

	HandleInitialContextSetupResponse(ctx, amfInstance, ran, resp)
}

// receiveInitialContextSetupFailure parses and handles an INITIAL CONTEXT SETUP
// FAILURE.
func receiveInitialContextSetupFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, uo *ngap.UnsuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, initialContextSetupFailureMessageType, span)

	fail, err := ngap.ParseInitialContextSetupFailure(uo.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Initial Context Setup Failure", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcInitialContextSetup, err)

		return
	}

	HandleInitialContextSetupFailure(ctx, amfInstance, ran, fail)
}

// receiveUERadioCapabilityInfoIndication parses and handles a UE RADIO
// CAPABILITY INFO INDICATION. The procedure defines no outcome, so a failed
// parse is answered with an Error Indication (TS 38.413 §10.3.5).
func receiveUERadioCapabilityInfoIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, ueRadioCapabilityInfoIndicationMessageType, span)

	ind, err := ngap.ParseUERadioCapabilityInfoIndication(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode UE Radio Capability Info Indication", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcUERadioCapabilityInfoIndication, err)

		return
	}

	HandleUERadioCapabilityInfoIndication(ctx, amfInstance, ran, ind)
}

// receivePDUSessionResourceSetupResponse parses and handles a PDU SESSION
// RESOURCE SETUP RESPONSE. A failed parse is answered with an Error Indication:
// the AMF started the procedure, so it cannot report the fault through the
// unsuccessful outcome (TS 38.413 §10.3.5).
func receivePDUSessionResourceSetupResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, so *ngap.SuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, pduSessionResourceSetupResponseMessageType, span)

	resp, err := ngap.ParsePDUSessionResourceSetupResponse(so.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode PDU Session Resource Setup Response", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcPDUSessionResourceSetup, err)

		return
	}

	HandlePDUSessionResourceSetupResponse(ctx, amfInstance, ran, resp)
}

// receivePDUSessionResourceReleaseResponse parses and handles a PDU SESSION
// RESOURCE RELEASE RESPONSE (TS 38.413 §10.3.5).
func receivePDUSessionResourceReleaseResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, so *ngap.SuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, pduSessionResourceReleaseResponseMessageType, span)

	resp, err := ngap.ParsePDUSessionResourceReleaseResponse(so.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode PDU Session Resource Release Response", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcPDUSessionResourceRelease, err)

		return
	}

	HandlePDUSessionResourceReleaseResponse(ctx, amfInstance, ran, resp)
}

// receivePDUSessionResourceModifyResponse parses and handles a PDU SESSION
// RESOURCE MODIFY RESPONSE (TS 38.413 §10.3.5).
func receivePDUSessionResourceModifyResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, so *ngap.SuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, pduSessionResourceModifyResponseMessageType, span)

	resp, err := ngap.ParsePDUSessionResourceModifyResponse(so.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode PDU Session Resource Modify Response", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcPDUSessionResourceModify, err)

		return
	}

	HandlePDUSessionResourceModifyResponse(ctx, amfInstance, ran, resp)
}

// receivePDUSessionResourceModifyIndication parses and handles a PDU SESSION
// RESOURCE MODIFY INDICATION (TS 38.413 §10.3.5).
func receivePDUSessionResourceModifyIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, pduSessionResourceModifyIndicationMessageType, span)

	ind, err := ngap.ParsePDUSessionResourceModifyIndication(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode PDU Session Resource Modify Indication", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcPDUSessionResourceModifyIndication, err)

		return
	}

	HandlePDUSessionResourceModifyIndication(ctx, amfInstance, ran, ind)
}

// receiveHandoverCancel parses and handles a HANDOVER CANCEL (TS 38.413
// §8.4.5).
func receiveHandoverCancel(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, handoverCancelMessageType, span)

	cancel, err := ngap.ParseHandoverCancel(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Handover Cancel", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcHandoverCancel, err)

		return
	}

	HandleHandoverCancel(ctx, amfInstance, ran, cancel)
}

func receiveHandoverNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, handoverNotifyMessageType, span)

	notify, err := ngap.ParseHandoverNotify(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Handover Notify", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcHandoverNotification, err)

		return
	}

	HandleHandoverNotify(ctx, amfInstance, ran, notify)
}

// receiveHandoverRequired parses and handles a HANDOVER REQUIRED. Handover
// Preparation defines an unsuccessful outcome, so §10.3.4.2 answers a rejected
// message with HANDOVER PREPARATION FAILURE rather than an Error Indication —
// but only when the rejected message still yielded the UE NGAP IDs that message
// needs.
func receiveHandoverRequired(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, handoverRequiredMessageType, span)

	required, err := ngap.ParseHandoverRequired(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Handover Required", zap.Error(err))

		var ase *ngap.AbstractSyntaxError
		if errors.As(err, &ase) {
			if amfID, ranID := ase.UEIDs(); amfID != nil && ranID != nil {
				sendHandoverPreparationProtocolFailure(ctx, ran, *amfID, *ranID, ase)

				return
			}
		}

		sendParseErrorIndication(ctx, ran, ngap.ProcHandoverPreparation, err)

		return
	}

	HandleHandoverRequired(ctx, amfInstance, ran, required)
}

// receiveUplinkRANStatusTransfer parses and handles an UPLINK RAN STATUS
// TRANSFER. The procedure defines no unsuccessful outcome, so a failed parse is
// answered with an Error Indication (TS 38.413 §10.3.5).
func receiveUplinkRANStatusTransfer(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, uplinkRANStatusTransferMessageType, span)

	transfer, err := ngap.ParseUplinkRANStatusTransfer(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Uplink RAN Status Transfer", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcUplinkRANStatusTransfer, err)

		return
	}

	HandleUplinkRanStatusTransfer(ctx, amfInstance, ran, transfer)
}

// receiveHandoverRequestAcknowledge parses and handles a HANDOVER REQUEST
// ACKNOWLEDGE. §10.3.4.2 leaves a rejected response to local error handling, so
// a failed parse is only reported, not answered.
func receiveHandoverRequestAcknowledge(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, so *ngap.SuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, handoverRequestAcknowledgeMessageType, span)

	ack, err := ngap.ParseHandoverRequestAcknowledge(so.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Handover Request Acknowledge", zap.Error(err))
		return
	}

	HandleHandoverRequestAcknowledge(ctx, amfInstance, ran, ack)
}

// receiveHandoverFailure parses and handles a HANDOVER FAILURE. §10.3.4.2
// leaves a rejected response to local error handling, so a failed parse is only
// reported, not answered.
func receiveHandoverFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, uo *ngap.UnsuccessfulOutcome, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, handoverFailureMessageType, span)

	failure, err := ngap.ParseHandoverFailure(uo.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode Handover Failure", zap.Error(err))
		return
	}

	HandleHandoverFailure(ctx, amfInstance, ran, failure)
}

func receivePDUSessionResourceNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, im *ngap.InitiatingMessage, span trace.Span) {
	traceMessage(ctx, amfInstance, ran, msg, pduSessionResourceNotifyMessageType, span)

	notify, err := ngap.ParsePDUSessionResourceNotify(im.Value)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("failed to decode PDU Session Resource Notify", zap.Error(err))
		sendParseErrorIndication(ctx, ran, ngap.ProcPDUSessionResourceNotify, err)

		return
	}

	HandlePDUSessionResourceNotify(ctx, amfInstance, ran, notify)
}
