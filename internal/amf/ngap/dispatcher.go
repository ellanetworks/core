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
	case ngapType.ProcedureCodeAMFConfigurationUpdate:
		return "AMFConfigurationUpdate"
	case ngapType.ProcedureCodeAMFStatusIndication:
		return "AMFStatusIndication"
	case ngapType.ProcedureCodeCellTrafficTrace:
		return "CellTrafficTrace"
	case ngapType.ProcedureCodeDeactivateTrace:
		return "DeactivateTrace"
	case ngapType.ProcedureCodeDownlinkNASTransport:
		return "DownlinkNASTransport"
	case ngapType.ProcedureCodeDownlinkNonUEAssociatedNRPPaTransport:
		return "DownlinkNonUEAssociatedNRPPaTransport"
	case ngapType.ProcedureCodeDownlinkRANConfigurationTransfer:
		return "DownlinkRANConfigurationTransfer"
	case ngapType.ProcedureCodeDownlinkRANStatusTransfer:
		return "DownlinkRANStatusTransfer"
	case ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport:
		return "DownlinkUEAssociatedNRPPaTransport"
	case ngapType.ProcedureCodeErrorIndication:
		return "ErrorIndication"
	case ngapType.ProcedureCodeHandoverCancel:
		return "HandoverCancel"
	case ngapType.ProcedureCodeHandoverNotification:
		return "HandoverNotification"
	case ngapType.ProcedureCodeHandoverPreparation:
		return "HandoverPreparation"
	case ngapType.ProcedureCodeHandoverResourceAllocation:
		return "HandoverResourceAllocation"
	case ngapType.ProcedureCodeInitialContextSetup:
		return "InitialContextSetup"
	case ngapType.ProcedureCodeInitialUEMessage:
		return "InitialUEMessage"
	case ngapType.ProcedureCodeLocationReportingControl:
		return "LocationReportingControl"
	case ngapType.ProcedureCodeLocationReportingFailureIndication:
		return "LocationReportingFailureIndication"
	case ngapType.ProcedureCodeLocationReport:
		return "LocationReport"
	case ngapType.ProcedureCodeNASNonDeliveryIndication:
		return "NASNonDeliveryIndication"
	case ngapType.ProcedureCodeNGReset:
		return "NGReset"
	case ngapType.ProcedureCodeNGSetup:
		return "NGSetup"
	case ngapType.ProcedureCodeOverloadStart:
		return "OverloadStart"
	case ngapType.ProcedureCodeOverloadStop:
		return "OverloadStop"
	case ngapType.ProcedureCodePaging:
		return "Paging"
	case ngapType.ProcedureCodePathSwitchRequest:
		return "PathSwitchRequest"
	case ngapType.ProcedureCodePDUSessionResourceModify:
		return "PDUSessionResourceModify"
	case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
		return "PDUSessionResourceModifyIndication"
	case ngapType.ProcedureCodePDUSessionResourceRelease:
		return "PDUSessionResourceRelease"
	case ngapType.ProcedureCodePDUSessionResourceSetup:
		return "PDUSessionResourceSetup"
	case ngapType.ProcedureCodePDUSessionResourceNotify:
		return "PDUSessionResourceNotify"
	case ngapType.ProcedureCodePrivateMessage:
		return "PrivateMessage"
	case ngapType.ProcedureCodePWSCancel:
		return "PWSCancel"
	case ngapType.ProcedureCodePWSFailureIndication:
		return "PWSFailureIndication"
	case ngapType.ProcedureCodePWSRestartIndication:
		return "PWSRestartIndication"
	case ngapType.ProcedureCodeRANConfigurationUpdate:
		return "RANConfigurationUpdate"
	case ngapType.ProcedureCodeRerouteNASRequest:
		return "RerouteNASRequest"
	case ngapType.ProcedureCodeRRCInactiveTransitionReport:
		return "RRCInactiveTransitionReport"
	case ngapType.ProcedureCodeTraceFailureIndication:
		return "TraceFailureIndication"
	case ngapType.ProcedureCodeTraceStart:
		return "TraceStart"
	case ngapType.ProcedureCodeUEContextModification:
		return "UEContextModification"
	case ngapType.ProcedureCodeUEContextRelease:
		return "UEContextRelease"
	case ngapType.ProcedureCodeUEContextReleaseRequest:
		return "UEContextReleaseRequest"
	case ngapType.ProcedureCodeUERadioCapabilityCheck:
		return "UERadioCapabilityCheck"
	case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
		return "UERadioCapabilityInfoIndication"
	case ngapType.ProcedureCodeUETNLABindingRelease:
		return "UETNLABindingRelease"
	case ngapType.ProcedureCodeUplinkNASTransport:
		return "UplinkNASTransport"
	case ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport:
		return "UplinkNonUEAssociatedNRPPaTransport"
	case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
		return "UplinkRANConfigurationTransfer"
	case ngapType.ProcedureCodeUplinkRANStatusTransfer:
		return "UplinkRANStatusTransfer"
	case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
		return "UplinkUEAssociatedNRPPaTransport"
	case ngapType.ProcedureCodeWriteReplaceWarning:
		return "WriteReplaceWarning"
	case ngapType.ProcedureCodeSecondaryRATDataUsageReport:
		return "SecondaryRATDataUsageReport"
	default:
		return fmt.Sprintf("ProcedureCode%d", code)
	}
}

func Dispatch(conn net.Conn, msg []byte, ctext ctx.Context) {
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

	ranUe, _ := FetchRanUeContext(ran, pdu, ctext)

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

	spanName := fmt.Sprintf("AMF NGAP %s", procName)
	ctx, span := tracer.Start(ctx.Background(), spanName,
		trace.WithAttributes(
			attribute.String("net.peer", conn.RemoteAddr().String()),
			attribute.String("ngap.pdu_present", fmt.Sprintf("%d", pdu.Present)),
			attribute.String("ngap.procedureCode", procName),
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
			HandleNGReset(ran, pdu)
		case ngapType.ProcedureCodeHandoverCancel:
			HandleHandoverCancel(ran, pdu, ctx)
		case ngapType.ProcedureCodeUEContextReleaseRequest:
			HandleUEContextReleaseRequest(ran, pdu, ctx)
		case ngapType.ProcedureCodeNASNonDeliveryIndication:
			HandleNasNonDeliveryIndication(ctx, ran, pdu)
		case ngapType.ProcedureCodeLocationReportingFailureIndication:
			HandleLocationReportingFailureIndication(ran, pdu)
		case ngapType.ProcedureCodeErrorIndication:
			HandleErrorIndication(ran, pdu)
		case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
			HandleUERadioCapabilityInfoIndication(ran, pdu)
		case ngapType.ProcedureCodeHandoverNotification:
			HandleHandoverNotify(ran, pdu, ctx)
		case ngapType.ProcedureCodeHandoverPreparation:
			HandleHandoverRequired(ran, pdu, ctx)
		case ngapType.ProcedureCodeRANConfigurationUpdate:
			HandleRanConfigurationUpdate(ctx, ran, pdu)
		case ngapType.ProcedureCodeRRCInactiveTransitionReport:
			HandleRRCInactiveTransitionReport(ran, pdu, ctx)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			HandlePDUSessionResourceNotify(ran, pdu, ctx)
		case ngapType.ProcedureCodePathSwitchRequest:
			HandlePathSwitchRequest(ran, pdu, ctx)
		case ngapType.ProcedureCodeLocationReport:
			HandleLocationReport(ran, pdu, ctx)
		case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
			HandleUplinkUEAssociatedNRPPATransport(ran, pdu)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			HandleUplinkRanConfigurationTransfer(ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			HandlePDUSessionResourceModifyIndication(ran, pdu, ctx)
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
			HandleUEContextReleaseComplete(ran, pdu, ctx)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			HandlePDUSessionResourceReleaseResponse(ran, pdu, ctx)
		case ngapType.ProcedureCodeUERadioCapabilityCheck:
			HandleUERadioCapabilityCheckResponse(ran, pdu)
		case ngapType.ProcedureCodeAMFConfigurationUpdate:
			HandleAMFconfigurationUpdateAcknowledge(ran, pdu)
		case ngapType.ProcedureCodeInitialContextSetup:
			HandleInitialContextSetupResponse(ran, pdu, ctx)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationResponse(ran, pdu, ctx)
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			HandlePDUSessionResourceSetupResponse(ran, pdu, ctx)
		case ngapType.ProcedureCodePDUSessionResourceModify:
			HandlePDUSessionResourceModifyResponse(ran, pdu, ctx)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverRequestAcknowledge(ran, pdu, ctx)
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
			HandleInitialContextSetupFailure(ran, pdu, ctx)
		case ngapType.ProcedureCodeUEContextModification:
			HandleUEContextModificationFailure(ran, pdu)
		case ngapType.ProcedureCodeHandoverResourceAllocation:
			HandleHandoverFailure(ran, pdu, ctx)
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
