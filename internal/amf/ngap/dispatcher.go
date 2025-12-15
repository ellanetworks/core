// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	ctxt "context"
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/internal/amf/context"
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

func Dispatch(ctx ctxt.Context, conn *sctp.SCTPConn, msg []byte) {
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

	amfSelf := context.AMFSelf()

	ran, ok := amfSelf.AmfRanFindByConn(conn)
	if !ok {
		ran = amfSelf.NewAmfRan(conn)
		logger.AmfLog.Info("Added a new radio", zap.String("address", remoteAddress.String()))
	}

	if len(msg) == 0 {
		ran.Log.Info("RAN close the connection.")
		ran.Remove()
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

	go DispatchNgapMsg(ran, pdu)
}

func DispatchNgapMsg(ran *context.AmfRan, pdu *ngapType.NGAPPDU) {
	messageType := getMessageType(pdu)

	spanName := fmt.Sprintf("AMF NGAP %s", messageType)
	ctx, span := tracer.Start(ctxt.Background(), spanName,
		trace.WithAttributes(
			attribute.String("ngap.pdu_present", fmt.Sprintf("%d", pdu.Present)),
			attribute.String("ngap.messageType", messageType),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		initiatingMessage := pdu.InitiatingMessage
		if initiatingMessage == nil {
			ran.Log.Error("Initiating Message is nil")
			return
		}

		switch initiatingMessage.ProcedureCode.Value {
		case ngapType.ProcedureCodeNGSetup:
			HandleNGSetupRequest(ctx, ran, pdu)
		case ngapType.ProcedureCodeInitialUEMessage:
			HandleInitialUEMessage(ctx, ran, pdu)
		case ngapType.ProcedureCodeUplinkNASTransport:
			HandleUplinkNasTransport(ctx, ran, pdu)
		case ngapType.ProcedureCodeNGReset:
			HandleNGReset(ctx, ran, pdu)
		case ngapType.ProcedureCodeHandoverCancel:
			HandleHandoverCancel(ctx, ran, pdu)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			HandleUEContextReleaseRequest(ctx, ran, pdu)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			HandleNasNonDeliveryIndication(ctx, ran, pdu)
		case ngapType.ProcedureCodeErrorIndication:
			HandleErrorIndication(ran, pdu)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			HandleUERadioCapabilityInfoIndication(ran, pdu)
		case ngapType.ProcedureCodeHandoverNotification:
			HandleHandoverNotify(ctx, ran, pdu)
		case ngapType.ProcedureCodeHandoverPreparation:
			HandleHandoverRequired(ctx, ran, pdu)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			HandleRanConfigurationUpdate(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			HandlePDUSessionResourceNotify(ctx, ran, pdu)
		case ngapType.ProcedureCodePathSwitchRequest:
			HandlePathSwitchRequest(ctx, ran, pdu)
		case ngapType.ProcedureCodeLocationReport:
			HandleLocationReport(ctx, ran, pdu)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			HandleUplinkRanConfigurationTransfer(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			HandlePDUSessionResourceModifyIndication(ctx, ran, pdu)
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
			HandleUEContextReleaseComplete(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			HandlePDUSessionResourceReleaseResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			HandlePDUSessionResourceSetupResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			HandlePDUSessionResourceModifyResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverRequestAcknowledge(ctx, ran, pdu)
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
			HandleInitialContextSetupFailure(ctx, ran, pdu)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationFailure(ran, pdu)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverFailure(ctx, ran, pdu)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", unsuccessfulOutcome.ProcedureCode.Value))
		}
	}
}

func HandleSCTPNotification(conn *sctp.SCTPConn, notification sctp.Notification) {
	amfSelf := context.AMFSelf()

	ran, ok := amfSelf.AmfRanFindByConn(conn)
	if !ok {
		logger.AmfLog.Debug("couldn't find RAN context", zap.Any("address", conn.RemoteAddr()))
		return
	}

	amfSelf.Mutex.Lock()
	for _, amfRan := range amfSelf.AmfRanPool {
		errorConn := sctp.NewSCTPConn(-1, nil)
		if reflect.DeepEqual(amfRan.Conn, errorConn) {
			amfRan.Remove()
			ran.Log.Info("removed stale entry in AmfRan pool")
		}
	}
	amfSelf.Mutex.Unlock()

	switch notification.Type() {
	case sctp.SCTPAssocChange:
		ran.Log.Info("SCTPAssocChange notification")
		event := notification.(*sctp.SCTPAssocChangeEvent)
		switch event.State() {
		case sctp.SCTPCommLost:
			ran.Remove()
			ran.Log.Info("Closed connection with radio after SCTP Communication Lost")
		case sctp.SCTPShutdownComp:
			ran.Remove()
			ran.Log.Info("Closed connection with radio after SCTP Shutdown Complete")
		default:
			ran.Log.Info("SCTP state is not handled", zap.Int("state", int(event.State())))
		}
	case sctp.SCTPShutdownEvent:
		ran.Remove()
		ran.Log.Info("Closed connection with radio after SCTP Shutdown Event")
	default:
		ran.Log.Warn("Unhandled SCTP notification type", zap.Any("type", notification.Type()))
	}
}
