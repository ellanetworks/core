// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	ctx "context"
	"fmt"
	"net"
	"reflect"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/ngap")

func procedureName(code int64) string {
	switch code {
	case ngapType.ProcedureCodeNGSetup:
		return "NGSetup"
	case ngapType.ProcedureCodeInitialUEMessage:
		return "InitialUEMessage"
	case ngapType.ProcedureCodeUplinkNASTransport:
		return "UplinkNASTransport"
	case ngapType.ProcedureCodeNGReset:
		return "NGReset"
	// add other specific mappings as needed
	default:
		return fmt.Sprintf("ProcedureCode%d", code)
	}
}

func Dispatch(conn net.Conn, msg []byte) {
	var ran *context.AmfRan
	amfSelf := context.AMFSelf()

	ran, ok := amfSelf.AmfRanFindByConn(conn)
	if !ok {
		ran = amfSelf.NewAmfRan(conn)
		logger.AmfLog.Info("Added a new radio", zap.String("address", conn.RemoteAddr().String()))
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

	ranUe, _ := FetchRanUeContext(ran, pdu)

	/* uecontext is found, submit the message to transaction queue*/
	if ranUe != nil && ranUe.AmfUe != nil {
		ranUe.AmfUe.TxLog.Debug("Uecontext found. queuing ngap message to uechannel")
		ngapMsg := context.NgapMsg{
			Ran:     ran,
			NgapMsg: pdu,
		}
		ranUe.Ran.Conn = conn
		NgapMsgHandler(conn, ranUe.AmfUe, ngapMsg)
	} else {
		go DispatchNgapMsg(conn, ran, pdu)
	}
}

func NgapMsgHandler(conn net.Conn, ue *context.AmfUe, msg context.NgapMsg) {
	DispatchNgapMsg(conn, msg.Ran, msg.NgapMsg)
}

func DispatchNgapMsg(conn net.Conn, ran *context.AmfRan, pdu *ngapType.NGAPPDU) {
	var code int64
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		if pdu.InitiatingMessage != nil {
			code = pdu.InitiatingMessage.ProcedureCode.Value
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		if pdu.SuccessfulOutcome != nil {
			code = pdu.SuccessfulOutcome.ProcedureCode.Value
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		if pdu.UnsuccessfulOutcome != nil {
			code = pdu.UnsuccessfulOutcome.ProcedureCode.Value
		}
	}
	procName := procedureName(code)

	if procName == "" {
		procName = "UnknownProcedure"
	}

	// Start span named "ngap.<ProcedureName>"
	spanName := fmt.Sprintf("ngap.%s", procName)
	_, span := tracer.Start(ctx.Background(), spanName,
		trace.WithAttributes(
			attribute.String("net.peer", conn.RemoteAddr().String()),
			attribute.String("ngap.pdu_present", fmt.Sprintf("%d", pdu.Present)),
			attribute.String("ngap.procedureCode", procName),
			// attribute.Int("ngap.msg_length", len(pdu.Value)),
		),
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
			HandleNGSetupRequest(ran, pdu)
		case ngapType.ProcedureCodeInitialUEMessage:
			HandleInitialUEMessage(ran, pdu)
		case ngapType.ProcedureCodeUplinkNASTransport:
			HandleUplinkNasTransport(ran, pdu)
		case ngapType.ProcedureCodeNGReset:
			HandleNGReset(ran, pdu)
		case ngapType.ProcedureCodeHandoverCancel:
			HandleHandoverCancel(ran, pdu)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			HandleUEContextReleaseRequest(ran, pdu)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			HandleNasNonDeliveryIndication(ran, pdu)
		case ngapType.ProcedureCodeLocationReportingFailureIndication:
			HandleLocationReportingFailureIndication(ran, pdu)
		case ngapType.ProcedureCodeErrorIndication:
			HandleErrorIndication(ran, pdu)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			HandleUERadioCapabilityInfoIndication(ran, pdu)
		case ngapType.ProcedureCodeHandoverNotification:
			HandleHandoverNotify(ran, pdu)
		case ngapType.ProcedureCodeHandoverPreparation:
			HandleHandoverRequired(ran, pdu)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			HandleRanConfigurationUpdate(ran, pdu)
		case ngapType.ProcedureCodeRRCInactiveTransitionReport:
			HandleRRCInactiveTransitionReport(ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			HandlePDUSessionResourceNotify(ran, pdu)
		case ngapType.ProcedureCodePathSwitchRequest:
			HandlePathSwitchRequest(ran, pdu)
		case ngapType.ProcedureCodeLocationReport:
			HandleLocationReport(ran, pdu)
		case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
			HandleUplinkUEAssociatedNRPPATransport(ran, pdu)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			HandleUplinkRanConfigurationTransfer(ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			HandlePDUSessionResourceModifyIndication(ran, pdu)
		case ngapType.ProcedureCodeCellTrafficTrace:
			HandleCellTrafficTrace(ran, pdu)
		case ngapType.ProcedureCodeUplinkRANStatusTransfer:
			HandleUplinkRanStatusTransfer(ran, pdu)
		case ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport:
			HandleUplinkNonUEAssociatedNRPPATransport(ran, pdu)
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
		case ngapType.ProcedureCodeNGReset:
			HandleNGResetAcknowledge(ran, pdu)
		case ngapType.ProcedureCodeUEContextRelease:
			HandleUEContextReleaseComplete(ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			HandlePDUSessionResourceReleaseResponse(ran, pdu)
		case ngapType.ProcedureCodeUERadioCapabilityCheck:
			HandleUERadioCapabilityCheckResponse(ran, pdu)
		case ngapType.ProcedureCodeAMFConfigurationUpdate:
			HandleAMFconfigurationUpdateAcknowledge(ran, pdu)
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupResponse(ran, pdu)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationResponse(ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			HandlePDUSessionResourceSetupResponse(ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			HandlePDUSessionResourceModifyResponse(ran, pdu)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverRequestAcknowledge(ran, pdu)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", successfulOutcome.ProcedureCode.Value))
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		unsuccessfulOutcome := pdu.UnsuccessfulOutcome
		if unsuccessfulOutcome == nil {
			ran.Log.Error("unsuccessful Outcome is nil")
			return
		}

		switch unsuccessfulOutcome.ProcedureCode.Value {
		case ngapType.ProcedureCodeAMFConfigurationUpdate:
			HandleAMFconfigurationUpdateFailure(ran, pdu)
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupFailure(ran, pdu)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationFailure(ran, pdu)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverFailure(ran, pdu)
		default:
			ran.Log.Warn("Not implemented", zap.Int("choice", pdu.Present), zap.Int64("procedureCode", unsuccessfulOutcome.ProcedureCode.Value))
		}
	}
}

func HandleSCTPNotification(conn net.Conn, notification sctp.Notification) {
	amfSelf := context.AMFSelf()

	ran, ok := amfSelf.AmfRanFindByConn(conn)
	if !ok {
		logger.AmfLog.Warn("couldn't find RAN context", zap.Any("address", conn.RemoteAddr()))
		return
	}

	// Removing Stale Connections in AmfRanPool
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		amfRan := value.(*context.AmfRan)

		conn := amfRan.Conn.(*sctp.SCTPConn)
		errorConn := sctp.NewSCTPConn(-1, nil)
		if reflect.DeepEqual(conn, errorConn) {
			amfRan.Remove()
			ran.Log.Info("removed stale entry in AmfRan pool")
		}
		return true
	})

	switch notification.Type() {
	case sctp.SCTPAssocChange:
		ran.Log.Info("SCTPAssocChange notification")
		event := notification.(*sctp.SCTPAssocChangeEvent)
		switch event.State() {
		case sctp.SCTPCommLost:
			ran.Log.Info("SCTP state is SCTPCommLost, close the connection")
			ran.Remove()
		case sctp.SCTPShutdownComp:
			ran.Log.Info("SCTP state is SCTPShutdownComp, close the connection")
			ran.Remove()
		default:
			ran.Log.Info("SCTP state is not handled", zap.Int("state", int(event.State())))
		}
	case sctp.SCTPShutdownEvent:
		ran.Log.Info("SCTPShutdownEvent notification, close the connection")
		ran.Remove()
	default:
		ran.Log.Warn("Non handled notification type", zap.Any("type", notification.Type()))
	}
}
