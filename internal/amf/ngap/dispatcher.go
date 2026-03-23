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

	// For NGSetupRequest the radio name is embedded in the message IEs and
	// hasn't been applied to ran.Name yet (that happens inside
	// HandleNGSetupRequest).  Peek at the decoded PDU so the inbound event
	// is logged with the correct radio name *before* dispatch — preserving
	// correct chronological ordering with the outbound NGSetupResponse.
	if name := ngSetupRadioName(pdu); name != "" {
		ran.Name = name
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

	dispatchNgapMsg(ctx, amfInstance, ran, pdu)
}

// ngSetupRadioName extracts the RANNodeName from an NGSetupRequest PDU.
// Returns "" for any other message type or if the name IE is absent.
func ngSetupRadioName(pdu *ngapType.NGAPPDU) string {
	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage ||
		pdu.InitiatingMessage == nil ||
		pdu.InitiatingMessage.ProcedureCode.Value != ngapType.ProcedureCodeNGSetup {
		return ""
	}

	req := pdu.InitiatingMessage.Value.NGSetupRequest
	if req == nil {
		return ""
	}

	for _, ie := range req.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDRANNodeName && ie.Value.RANNodeName != nil {
			return ie.Value.RANNodeName.Value
		}
	}

	return ""
}

func dispatchNgapMsg(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, pdu *ngapType.NGAPPDU) {
	NGAPMessages.WithLabelValues(getMessageType(pdu)).Inc()

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := pdu.InitiatingMessage
		if initiatingMessage == nil {
			ran.Log.Error("Initiating Message is nil")
			return
		}

		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			HandleNGSetupRequest(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.NGSetupRequest)
		case ngapType.ProcedureCodeInitialUEMessage:
			HandleInitialUEMessage(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.InitialUEMessage)
		case ngapType.ProcedureCodeUplinkNASTransport:
			HandleUplinkNasTransport(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.UplinkNASTransport)
		case ngapType.ProcedureCodeNGReset:
			HandleNGReset(ctx, ran, pdu.InitiatingMessage.Value.NGReset)
		case ngapType.ProcedureCodeHandoverCancel:
			HandleHandoverCancel(ctx, ran, pdu.InitiatingMessage.Value.HandoverCancel)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			HandleUEContextReleaseRequest(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.UEContextReleaseRequest)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			HandleNasNonDeliveryIndication(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.NASNonDeliveryIndication)
		case ngapType.ProcedureCodeErrorIndication:
			HandleErrorIndication(ctx, ran, pdu.InitiatingMessage.Value.ErrorIndication)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			HandleUERadioCapabilityInfoIndication(ctx, ran, pdu.InitiatingMessage.Value.UERadioCapabilityInfoIndication)
		case ngapType.ProcedureCodeHandoverNotification:
			HandleHandoverNotify(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.HandoverNotify)
		case ngapType.ProcedureCodeHandoverPreparation:
			HandleHandoverRequired(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.HandoverRequired)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			HandleRanConfigurationUpdate(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.RANConfigurationUpdate)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			HandlePDUSessionResourceNotify(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.PDUSessionResourceNotify)
		case ngapType.ProcedureCodePathSwitchRequest:
			HandlePathSwitchRequest(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.PathSwitchRequest)
		case ngapType.ProcedureCodeLocationReport:
			HandleLocationReport(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.LocationReport)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			HandleUplinkRanConfigurationTransfer(ctx, amfInstance, ran, pdu.InitiatingMessage.Value.UplinkRANConfigurationTransfer)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			HandlePDUSessionResourceModifyIndication(ctx, ran, pdu.InitiatingMessage.Value.PDUSessionResourceModifyIndication)
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
			HandleUEContextReleaseComplete(ctx, amfInstance, ran, pdu.SuccessfulOutcome.Value.UEContextReleaseComplete)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			HandlePDUSessionResourceReleaseResponse(ctx, amfInstance, ran, pdu.SuccessfulOutcome.Value.PDUSessionResourceReleaseResponse)
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupResponse(ctx, amfInstance, ran, pdu.SuccessfulOutcome.Value.InitialContextSetupResponse)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationResponse(ctx, amfInstance, ran, pdu.SuccessfulOutcome.Value.UEContextModificationResponse)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			HandlePDUSessionResourceSetupResponse(ctx, amfInstance, ran, pdu.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			HandlePDUSessionResourceModifyResponse(ctx, amfInstance, ran, pdu.SuccessfulOutcome.Value.PDUSessionResourceModifyResponse)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverRequestAcknowledge(ctx, amfInstance, ran, pdu.SuccessfulOutcome.Value.HandoverRequestAcknowledge)
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
			HandleInitialContextSetupFailure(ctx, amfInstance, ran, pdu.UnsuccessfulOutcome.Value.InitialContextSetupFailure)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationFailure(ctx, amfInstance, ran, pdu.UnsuccessfulOutcome.Value.UEContextModificationFailure)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverFailure(ctx, amfInstance, ran, pdu.UnsuccessfulOutcome.Value.HandoverFailure)
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
