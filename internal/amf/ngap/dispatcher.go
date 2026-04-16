// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap"
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
	if remoteAddress == nil {
		logger.AmfLog.Debug("Remote address is nil")
		return
	}

	localAddress := conn.LocalAddr()
	if localAddress == nil {
		logger.AmfLog.Debug("Local address is nil")
		return
	}

	ran, ok := amfInstance.FindRadioByConn(conn)
	if !ok {
		var err error

		ran, err = amfInstance.NewRadio(conn)
		if err != nil {
			logger.AmfLog.Error("Failed to add a new radio", zap.Error(err))
			return
		}

		logger.AmfLog.Info("Added a new radio", zap.String("address", remoteAddress.String()))
	}

	if len(msg) == 0 {
		ran.Log.Info("RAN close the connection.")
		amfInstance.RemoveRadio(ran)

		return
	}

	ran.TouchLastSeen()

	ctx, span := tracer.Start(ctx, "ngap/receive",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	pdu, err := ngap.Decoder(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode NGAP message")
		ran.Log.Error("NGAP decode error", zap.Error(err))

		return
	}

	messageType := getMessageType(pdu)

	span.SetAttributes(
		attribute.String("ngap.message_type", messageType),
		attribute.Int("ngap.pdu_category", pdu.Present),
		attribute.Int("ngap.message_size", len(msg)),
		attribute.String("network.protocol.name", "ngap"),
		attribute.String("network.transport", "sctp"),
		attribute.String("network.peer.address", remoteAddress.String()),
		attribute.String("network.local.address", localAddress.String()),
	)

	// Pre-decode NGSetupRequest so the peer's RANNodeName can be applied to
	// ran.Name *before* the inbound event is logged, preserving chronological
	// ordering with the outbound NGSetupResponse.
	var (
		ngSetupDecoded decode.NGSetupRequest
		ngSetupReport  *decode.Report
		haveNGSetup    bool
	)

	if pdu.Present == ngapType.NGAPPDUPresentInitiatingMessage &&
		pdu.InitiatingMessage != nil &&
		pdu.InitiatingMessage.ProcedureCode.Value == ngapType.ProcedureCodeNGSetup {
		ngSetupDecoded, ngSetupReport = decode.DecodeNGSetupRequest(pdu.InitiatingMessage.Value.NGSetupRequest)
		haveNGSetup = true

		if ngSetupDecoded.RANNodeName != "" {
			ran.Name = ngSetupDecoded.RANNodeName
		}
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		messageType,
		logger.DirectionInbound,
		localAddress.String(),
		remoteAddress.String(),
		ran.Name,
		msg,
	)

	NGAPMessages.WithLabelValues(messageType).Inc()

	if haveNGSetup {
		if !handleDecodeReport(ctx, ran, ngSetupReport) {
			return
		}

		HandleNGSetupRequest(ctx, amfInstance, ran, ngSetupDecoded)

		return
	}

	// TS 38.413 §10.4: NG Setup must be the first NGAP procedure after
	// the TNL association is established. Reject anything else.
	if ran.RanID == nil {
		ran.Log.Error("Received NGAP message before NG Setup, dropping", zap.String("messageType", messageType))

		return
	}

	dispatchNgapMsg(ctx, amfInstance, ran, pdu)
}

func dispatchNgapMsg(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, pdu *ngapType.NGAPPDU) {
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := pdu.InitiatingMessage
		if initiatingMessage == nil {
			ran.Log.Error("Initiating Message is nil")
			return
		}

		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeInitialUEMessage:
			decoded, report := decode.DecodeInitialUEMessage(pdu.InitiatingMessage.Value.InitialUEMessage)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleInitialUEMessage(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUplinkNASTransport:
			decoded, report := decode.DecodeUplinkNASTransport(pdu.InitiatingMessage.Value.UplinkNASTransport)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUplinkNasTransport(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeNGReset:
			decoded, report := decode.DecodeNGReset(pdu.InitiatingMessage.Value.NGReset)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleNGReset(ctx, ran, decoded)
		case ngapType.ProcedureCodeHandoverCancel:
			decoded, report := decode.DecodeHandoverCancel(pdu.InitiatingMessage.Value.HandoverCancel)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverCancel(ctx, ran, decoded)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			decoded, report := decode.DecodeUEContextReleaseRequest(pdu.InitiatingMessage.Value.UEContextReleaseRequest)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUEContextReleaseRequest(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			decoded, report := decode.DecodeNASNonDeliveryIndication(pdu.InitiatingMessage.Value.NASNonDeliveryIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleNasNonDeliveryIndication(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeErrorIndication:
			decoded, report := decode.DecodeErrorIndication(pdu.InitiatingMessage.Value.ErrorIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleErrorIndication(ctx, ran, decoded)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			decoded, report := decode.DecodeUERadioCapabilityInfoIndication(pdu.InitiatingMessage.Value.UERadioCapabilityInfoIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUERadioCapabilityInfoIndication(ctx, ran, decoded)
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
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			decoded, report := decode.DecodeRANConfigurationUpdate(pdu.InitiatingMessage.Value.RANConfigurationUpdate)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleRanConfigurationUpdate(ctx, amfInstance, ran, decoded)
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
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			decoded, report := decode.DecodeUplinkRANConfigurationTransfer(pdu.InitiatingMessage.Value.UplinkRANConfigurationTransfer)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUplinkRanConfigurationTransfer(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			decoded, report := decode.DecodePDUSessionResourceModifyIndication(pdu.InitiatingMessage.Value.PDUSessionResourceModifyIndication)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandlePDUSessionResourceModifyIndication(ctx, ran, decoded)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", initiatingMessage.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		successfulOutcome := pdu.SuccessfulOutcome
		if successfulOutcome == nil {
			ran.Log.Error("successful Outcome is nil")
			return
		}

		switch successfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeUEContextRelease:
			decoded, report := decode.DecodeUEContextReleaseComplete(pdu.SuccessfulOutcome.Value.UEContextReleaseComplete)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUEContextReleaseComplete(ctx, amfInstance, ran, decoded)
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
		case ngapType.ProcedureCodeUEContextModification:
			decoded, report := decode.DecodeUEContextModificationResponse(pdu.SuccessfulOutcome.Value.UEContextModificationResponse)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUEContextModificationResponse(ctx, amfInstance, ran, decoded)
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
			ran.Log.Warn("NGAP Message handler not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", successfulOutcome.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		unsuccessfulOutcome := pdu.UnsuccessfulOutcome
		if unsuccessfulOutcome == nil {
			ran.Log.Error("unsuccessful Outcome is nil")
			return
		}

		switch unsuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeInitialContextSetup:
			decoded, report := decode.DecodeInitialContextSetupFailure(pdu.UnsuccessfulOutcome.Value.InitialContextSetupFailure)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleInitialContextSetupFailure(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeUEContextModification:
			decoded, report := decode.DecodeUEContextModificationFailure(pdu.UnsuccessfulOutcome.Value.UEContextModificationFailure)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleUEContextModificationFailure(ctx, amfInstance, ran, decoded)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			decoded, report := decode.DecodeHandoverFailure(pdu.UnsuccessfulOutcome.Value.HandoverFailure)
			if !handleDecodeReport(ctx, ran, report) {
				return
			}

			HandleHandoverFailure(ctx, amfInstance, ran, decoded)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", unsuccessfulOutcome.ProcedureCode.Value))
		}
	}
}

func HandleSCTPNotification(amfInstance *amf.AMF, conn *sctp.SCTPConn, notification sctp.Notification) {
	ran, ok := amfInstance.FindRadioByConn(conn)
	if !ok {
		logger.AmfLog.Debug("couldn't find RAN context", zap.Any("address", conn.RemoteAddr()))
		return
	}

	switch notification.Type() {
	case sctp.SCTPAssocChange:
		ran.Log.Info("SCTPAssocChange notification")

		event := notification.(*sctp.SCTPAssocChangeEvent)
		if event.Info() != nil {
			ran.Log.Info("SCTP assoc change info",
				zap.Binary("info", event.Info()),
				zap.Uint16("error", event.Error()))
		}

		switch event.State() {
		case sctp.SCTPCommLost:
			amfInstance.RemoveRadio(ran)
			ran.Log.Info("Closed connection with radio after SCTP Communication Lost")
		case sctp.SCTPShutdownComp:
			amfInstance.RemoveRadio(ran)
			ran.Log.Info("Closed connection with radio after SCTP Shutdown Complete")
		default:
			ran.Log.Info("SCTP state is not handled", zap.Int("state", int(event.State())))
		}
	case sctp.SCTPShutdownEvent:
		amfInstance.RemoveRadio(ran)
		ran.Log.Info("Closed connection with radio after SCTP Shutdown Event")
	default:
		ran.Log.Warn("Unhandled SCTP notification type", zap.Any("type", notification.Type()))
	}
}
