// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
	free5gcngap "github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/ngap")

func Dispatch(ctx context.Context, amfInstance *amf.AMF, conn *sctp.SCTPConn, msg []byte) {
	remoteAddress := conn.RemoteAddr()
	localAddress := conn.LocalAddr()

	ran, ok := amfInstance.FindRadioByConn(conn)
	if !ok {
		var err error

		ran, err = amfInstance.NewRadio(conn)
		if err != nil {
			logger.AmfLog.Error("Failed to add a new radio", zap.Error(err))
			return
		}

		logger.AmfLog.Info("Added a new radio", zap.String("address", amf.AddrString(remoteAddress)))
	}

	if len(msg) == 0 {
		logger.From(ctx, ran.Log).Info("RAN close the connection.")
		amfInstance.RemoveRadio(ctx, ran)

		return
	}

	ran.TouchLastSeen()

	ctx, span := tracer.Start(ctx, "ngap/receive",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	// NG Setup is decoded by the in-house NGAP library. It is intercepted
	// before the free5gc decoder so exactly one codec sees the message; the
	// remaining procedures follow as they are migrated.
	if handled := handleMigrated(ctx, amfInstance, ran, msg, span); handled {
		return
	}

	pdu, err := free5gcngap.Decoder(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode NGAP message")
		logger.From(ctx, ran.Log).Error("NGAP decode error", zap.Error(err))
		sendProtocolErrorIndication(ctx, ran, ngap.CauseProtocolTransferSyntaxError)

		return
	}

	messageType := getMessageType(pdu)

	span.SetAttributes(
		attribute.String("ngap.message_type", string(messageType)),
		attribute.Int("ngap.pdu_category", pdu.Present),
		attribute.Int("ngap.message_size", len(msg)),
		attribute.String("network.protocol.name", "ngap"),
		attribute.String("network.transport", "sctp"),
		attribute.String("network.peer.address", amf.AddrString(remoteAddress)),
		attribute.String("network.local.address", amf.AddrString(localAddress)),
	)

	amfInstance.LogNetworkEvent(ctx, ran.Conn, messageType, logger.DirectionInbound, msg)

	// TS 38.413: NG Setup must be the first NGAP procedure after
	// the TNL association is established. Reject anything else.
	if ran.RanID == nil {
		logger.From(ctx, ran.Log).Error("Received NGAP message before NG Setup, dropping", zap.String("messageType", string(messageType)))

		return
	}

	dispatchNgapMsg(ctx, amfInstance, ran, pdu)
}

func dispatchNgapMsg(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, pdu *ngapType.NGAPPDU) {
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := pdu.InitiatingMessage
		if initiatingMessage == nil {
			logger.From(ctx, ran.Log).Error("Initiating Message is nil")
			return
		}

		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeHandoverCancel:
			decoded, report := decode.DecodeHandoverCancel(pdu.InitiatingMessage.Value.HandoverCancel)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverCancel(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			decoded, report := decode.DecodeUERadioCapabilityInfoIndication(pdu.InitiatingMessage.Value.UERadioCapabilityInfoIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUERadioCapabilityInfoIndication(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverNotification:
			decoded, report := decode.DecodeHandoverNotify(pdu.InitiatingMessage.Value.HandoverNotify)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverNotify(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverPreparation:
			decoded, report := decode.DecodeHandoverRequired(pdu.InitiatingMessage.Value.HandoverRequired)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverRequired(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUplinkRANStatusTransfer:
			decoded, report := decode.DecodeUplinkRANStatusTransfer(pdu.InitiatingMessage.Value.UplinkRANStatusTransfer)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUplinkRanStatusTransfer(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			decoded, report := decode.DecodePDUSessionResourceNotify(pdu.InitiatingMessage.Value.PDUSessionResourceNotify)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceNotify(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePathSwitchRequest:
			decoded, report := decode.DecodePathSwitchRequest(pdu.InitiatingMessage.Value.PathSwitchRequest)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePathSwitchRequest(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeLocationReport:
			decoded, report := decode.DecodeLocationReport(pdu.InitiatingMessage.Value.LocationReport)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleLocationReport(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			decoded, report := decode.DecodePDUSessionResourceModifyIndication(pdu.InitiatingMessage.Value.PDUSessionResourceModifyIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceModifyIndication(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
			HandleUplinkUEAssociatedNRPPaTransport(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.UplinkUEAssociatedNRPPaTransport)
		default:
			logger.From(ctx, ran.Log).Warn("unsupported initiating procedure", zap.Int64("procedureCode", initiatingMessage.ProcedureCode.Value))
			respondToUnknownProcedure(ctx, ran, initiatingMessage)
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		successfulOutcome := pdu.SuccessfulOutcome
		if successfulOutcome == nil {
			logger.From(ctx, ran.Log).Error("successful Outcome is nil")
			return
		}

		switch successfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			decoded, report := decode.DecodePDUSessionResourceReleaseResponse(pdu.SuccessfulOutcome.Value.PDUSessionResourceReleaseResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceReleaseResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeInitialContextSetup:
			decoded, report := decode.DecodeInitialContextSetupResponse(pdu.SuccessfulOutcome.Value.InitialContextSetupResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleInitialContextSetupResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			decoded, report := decode.DecodePDUSessionResourceSetupResponse(pdu.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceSetupResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			decoded, report := decode.DecodePDUSessionResourceModifyResponse(pdu.SuccessfulOutcome.Value.PDUSessionResourceModifyResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceModifyResponse(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			decoded, report := decode.DecodeHandoverRequestAcknowledge(pdu.SuccessfulOutcome.Value.HandoverRequestAcknowledge)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverRequestAcknowledge(ctx, amfInstance, ran, decoded)
		default:
			logger.From(ctx, ran.Log).Warn("ignoring unsupported procedure", zap.String("kind", "successful-outcome"), zap.Int64("procedureCode", successfulOutcome.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		unsuccessfulOutcome := pdu.UnsuccessfulOutcome
		if unsuccessfulOutcome == nil {
			logger.From(ctx, ran.Log).Error("unsuccessful Outcome is nil")
			return
		}

		switch unsuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeInitialContextSetup:
			decoded, report := decode.DecodeInitialContextSetupFailure(pdu.UnsuccessfulOutcome.Value.InitialContextSetupFailure)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleInitialContextSetupFailure(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			decoded, report := decode.DecodeHandoverFailure(pdu.UnsuccessfulOutcome.Value.HandoverFailure)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverFailure(ctx, amfInstance, ran, decoded)
		default:
			logger.From(ctx, ran.Log).Warn("ignoring unsupported procedure", zap.String("kind", "unsuccessful-outcome"), zap.Int64("procedureCode", unsuccessfulOutcome.ProcedureCode.Value))
		}
	}
}
