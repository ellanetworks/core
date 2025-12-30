// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"context"
	"fmt"
	"reflect"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/ngap")

func Dispatch(ctx context.Context, conn *sctp.SCTPConn, msg []byte) {
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

	amf := amfContext.AMFSelf()

	ran, ok := amf.FindRadioByConn(conn)
	if !ok {
		var err error
		ran, err = amf.NewRadio(conn)
		if err != nil {
			logger.AmfLog.Error("Failed to add a new radio", zap.Error(err))
			return
		}
		logger.AmfLog.Info("Added a new radio", zap.String("address", remoteAddress.String()))
	}

	if len(msg) == 0 {
		ran.Log.Info("RAN close the connection.")
		amf.RemoveRadio(ran)
		return
	}

	pdu, err := ngap.Decoder(msg)
	if err != nil {
		ran.Log.Error("NGAP decode error", zap.Error(err))
		return
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		getMessageType(pdu),
		logger.DirectionInbound,
		localAddress.String(),
		remoteAddress.String(),
		msg,
	)

	DispatchNgapMsg(ran, pdu)
}

func DispatchNgapMsg(ran *amfContext.Radio, pdu *ngapType.NGAPPDU) {
	messageType := getMessageType(pdu)

	spanName := fmt.Sprintf("AMF NGAP %s", messageType)
	ctx, span := tracer.Start(context.Background(), spanName,
		trace.WithAttributes(
			attribute.String("ngap.pdu_present", fmt.Sprintf("%d", pdu.Present)),
			attribute.String("ngap.messageType", messageType),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	amf := amfContext.AMFSelf()

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := pdu.InitiatingMessage
		if initiatingMessage == nil {
			ran.Log.Error("Initiating Message is nil")
			return
		}

		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			HandleNGSetupRequest(ctx, amf, ran, pdu.InitiatingMessage.Value.NGSetupRequest)
		case ngapType.ProcedureCodeInitialUEMessage:
			HandleInitialUEMessage(ctx, ran, pdu.InitiatingMessage.Value.InitialUEMessage)
		case ngapType.ProcedureCodeUplinkNASTransport:
			HandleUplinkNasTransport(ctx, ran, pdu.InitiatingMessage.Value.UplinkNASTransport)
		case ngapType.ProcedureCodeNGReset:
			HandleNGReset(ctx, ran, pdu.InitiatingMessage.Value.NGReset)
		case ngapType.ProcedureCodeHandoverCancel:
			HandleHandoverCancel(ctx, ran, pdu.InitiatingMessage.Value.HandoverCancel)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			HandleUEContextReleaseRequest(ctx, ran, pdu.InitiatingMessage.Value.UEContextReleaseRequest)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			HandleNasNonDeliveryIndication(ctx, ran, pdu.InitiatingMessage.Value.NASNonDeliveryIndication)
		case ngapType.ProcedureCodeErrorIndication:
			HandleErrorIndication(ran, pdu.InitiatingMessage.Value.ErrorIndication)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			HandleUERadioCapabilityInfoIndication(ran, pdu.InitiatingMessage.Value.UERadioCapabilityInfoIndication)
		case ngapType.ProcedureCodeHandoverNotification:
			HandleHandoverNotify(ctx, ran, pdu.InitiatingMessage.Value.HandoverNotify)
		case ngapType.ProcedureCodeHandoverPreparation:
			HandleHandoverRequired(ctx, ran, pdu.InitiatingMessage.Value.HandoverRequired)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			HandleRanConfigurationUpdate(ctx, ran, pdu.InitiatingMessage.Value.RANConfigurationUpdate)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			HandlePDUSessionResourceNotify(ctx, ran, pdu.InitiatingMessage.Value.PDUSessionResourceNotify)
		case ngapType.ProcedureCodePathSwitchRequest:
			HandlePathSwitchRequest(ctx, ran, pdu.InitiatingMessage.Value.PathSwitchRequest)
		case ngapType.ProcedureCodeLocationReport:
			HandleLocationReport(ctx, ran, pdu.InitiatingMessage.Value.LocationReport)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			HandleUplinkRanConfigurationTransfer(ctx, ran, pdu.InitiatingMessage.Value.UplinkRANConfigurationTransfer)
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
			HandleUEContextReleaseComplete(ctx, ran, pdu.SuccessfulOutcome.Value.UEContextReleaseComplete)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			HandlePDUSessionResourceReleaseResponse(ctx, ran, pdu.SuccessfulOutcome.Value.PDUSessionResourceReleaseResponse)
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupResponse(ctx, ran, pdu.SuccessfulOutcome.Value.InitialContextSetupResponse)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationResponse(ctx, ran, pdu.SuccessfulOutcome.Value.UEContextModificationResponse)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			HandlePDUSessionResourceSetupResponse(ctx, ran, pdu.SuccessfulOutcome.Value.PDUSessionResourceSetupResponse)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			HandlePDUSessionResourceModifyResponse(ctx, ran, pdu.SuccessfulOutcome.Value.PDUSessionResourceModifyResponse)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverRequestAcknowledge(ctx, ran, pdu.SuccessfulOutcome.Value.HandoverRequestAcknowledge)
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
			HandleInitialContextSetupFailure(ctx, ran, pdu.UnsuccessfulOutcome.Value.InitialContextSetupFailure)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationFailure(ran, pdu.UnsuccessfulOutcome.Value.UEContextModificationFailure)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverFailure(ctx, ran, pdu.UnsuccessfulOutcome.Value.HandoverFailure)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", unsuccessfulOutcome.ProcedureCode.Value))
		}
	}
}

func HandleSCTPNotification(conn *sctp.SCTPConn, notification sctp.Notification) {
	amf := amfContext.AMFSelf()

	ran, ok := amf.FindRadioByConn(conn)
	if !ok {
		logger.AmfLog.Debug("couldn't find RAN context", zap.Any("address", conn.RemoteAddr()))
		return
	}

	amf.Mutex.Lock()
	for _, amfRan := range amf.Radios {
		errorConn := sctp.NewSCTPConn(-1, nil)
		if reflect.DeepEqual(amfRan.Conn, errorConn) {
			amf.RemoveRadio(ran)
			ran.Log.Info("removed stale entry in AmfRan pool")
		}
	}
	amf.Mutex.Unlock()

	switch notification.Type() {
	case sctp.SCTPAssocChange:
		ran.Log.Info("SCTPAssocChange notification")
		event := notification.(*sctp.SCTPAssocChangeEvent)
		switch event.State() {
		case sctp.SCTPCommLost:
			amf.RemoveRadio(ran)
			ran.Log.Info("Closed connection with radio after SCTP Communication Lost")
		case sctp.SCTPShutdownComp:
			amf.RemoveRadio(ran)
			ran.Log.Info("Closed connection with radio after SCTP Shutdown Complete")
		default:
			ran.Log.Info("SCTP state is not handled", zap.Int("state", int(event.State())))
		}
	case sctp.SCTPShutdownEvent:
		amf.RemoveRadio(ran)
		ran.Log.Info("Closed connection with radio after SCTP Shutdown Event")
	default:
		ran.Log.Warn("Unhandled SCTP notification type", zap.Any("type", notification.Type()))
	}
}
