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

type NgapMsg struct {
	NgapMsg *ngapType.NGAPPDU
	Ran     *context.AmfRan
}

func Dispatch(ctx ctxt.Context, conn *sctp.SCTPConn, msg []byte) {
	var ran *context.AmfRan
	amfSelf := context.AMFSelf()

	remoteAddress := conn.RemoteAddr()
	if remoteAddress == nil {
		logger.AmfLog.Debug("Remote address is nil")
		return
	}

	ran, ok := amfSelf.AmfRanFindByConn(conn)
	if !ok {
		ran = amfSelf.NewAmfRan(conn)
		logger.AmfLog.Info("Added a new radio", zap.String("address", remoteAddress.String()))
	}

	localAddress := conn.LocalAddr()
	if localAddress == nil {
		logger.AmfLog.Debug("Local address is nil")
		return
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

	ranUe, _ := FetchRanUeContext(ctx, ran, pdu)

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		getMessageType(pdu),
		logger.DirectionInbound,
		localAddress.String(),
		remoteAddress.String(),
		msg,
	)

	/* uecontext is found, submit the message to transaction queue*/
	if ranUe != nil && ranUe.AmfUe != nil {
		ranUe.AmfUe.TxLog.Debug("Uecontext found, dispatching NGAP message")
		ngapMsg := NgapMsg{
			Ran:     ran,
			NgapMsg: pdu,
		}
		ranUe.Ran.Conn = conn
		NgapMsgHandler(conn, ranUe.AmfUe, ngapMsg)
	} else {
		go DispatchNgapMsg(conn, ran, pdu)
	}
}

func getMessageType(pdu *ngapType.NGAPPDU) string {
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		if pdu.InitiatingMessage != nil {
			return getInitiatingMessageType(pdu.InitiatingMessage.Value.Present)
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		if pdu.SuccessfulOutcome != nil {
			return getSuccessfulOutcomeMessageType(pdu.SuccessfulOutcome.Value.Present)
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		if pdu.UnsuccessfulOutcome != nil {
			return getUnsuccessfulOutcomeMessageType(pdu.UnsuccessfulOutcome.Value.Present)
		}
	default:
		return "UnknownMessage"
	}

	return "UnknownMessage"
}

func getInitiatingMessageType(present int) string {
	switch present {
	case ngapType.InitiatingMessagePresentNothing:
		return "Nothing"
	case ngapType.InitiatingMessagePresentAMFConfigurationUpdate:
		return "AMFConfigurationUpdate"
	case ngapType.InitiatingMessagePresentHandoverCancel:
		return "HandoverCancel"
	case ngapType.InitiatingMessagePresentHandoverRequired:
		return "HandoverRequired"
	case ngapType.InitiatingMessagePresentHandoverRequest:
		return "HandoverRequest"
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return "InitialContextSetupRequest"
	case ngapType.InitiatingMessagePresentNGReset:
		return "NGReset"
	case ngapType.InitiatingMessagePresentNGSetupRequest:
		return "NGSetupRequest"
	case ngapType.InitiatingMessagePresentPathSwitchRequest:
		return "PathSwitchRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyRequest:
		return "PDUSessionResourceModifyRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyIndication:
		return "PDUSessionResourceModifyIndication"
	case ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand:
		return "PDUSessionResourceReleaseCommand"
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return "PDUSessionResourceSetupRequest"
	case ngapType.InitiatingMessagePresentPWSCancelRequest:
		return "PWSCancelRequest"
	case ngapType.InitiatingMessagePresentRANConfigurationUpdate:
		return "RANConfigurationUpdate"
	case ngapType.InitiatingMessagePresentUEContextModificationRequest:
		return "UEContextModificationRequest"
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return "UEContextReleaseCommand"
	case ngapType.InitiatingMessagePresentUERadioCapabilityCheckRequest:
		return "UERadioCapabilityCheckRequest"
	case ngapType.InitiatingMessagePresentWriteReplaceWarningRequest:
		return "WriteReplaceWarningRequest"
	case ngapType.InitiatingMessagePresentAMFStatusIndication:
		return "AMFStatusIndication"
	case ngapType.InitiatingMessagePresentCellTrafficTrace:
		return "CellTrafficTrace"
	case ngapType.InitiatingMessagePresentDeactivateTrace:
		return "DeactivateTrace"
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return "DownlinkNASTransport"
	case ngapType.InitiatingMessagePresentDownlinkNonUEAssociatedNRPPaTransport:
		return "DownlinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentDownlinkRANConfigurationTransfer:
		return "DownlinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentDownlinkRANStatusTransfer:
		return "DownlinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport:
		return "DownlinkUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentErrorIndication:
		return "ErrorIndication"
	case ngapType.InitiatingMessagePresentHandoverNotify:
		return "HandoverNotify"
	case ngapType.InitiatingMessagePresentInitialUEMessage:
		return "InitialUEMessage"
	case ngapType.InitiatingMessagePresentLocationReport:
		return "LocationReport"
	case ngapType.InitiatingMessagePresentLocationReportingControl:
		return "LocationReportingControl"
	case ngapType.InitiatingMessagePresentLocationReportingFailureIndication:
		return "LocationReportingFailureIndication"
	case ngapType.InitiatingMessagePresentNASNonDeliveryIndication:
		return "NASNonDeliveryIndication"
	case ngapType.InitiatingMessagePresentOverloadStart:
		return "OverloadStart"
	case ngapType.InitiatingMessagePresentOverloadStop:
		return "OverloadStop"
	case ngapType.InitiatingMessagePresentPaging:
		return "Paging"
	case ngapType.InitiatingMessagePresentPDUSessionResourceNotify:
		return "PDUSessionResourceNotify"
	case ngapType.InitiatingMessagePresentPrivateMessage:
		return "PrivateMessage"
	case ngapType.InitiatingMessagePresentPWSFailureIndication:
		return "PWSFailureIndication"
	case ngapType.InitiatingMessagePresentPWSRestartIndication:
		return "PWSRestartIndication"
	case ngapType.InitiatingMessagePresentRerouteNASRequest:
		return "RerouteNASRequest"
	case ngapType.InitiatingMessagePresentRRCInactiveTransitionReport:
		return "RRCInactiveTransitionReport"
	case ngapType.InitiatingMessagePresentSecondaryRATDataUsageReport:
		return "SecondaryRATDataUsageReport"
	case ngapType.InitiatingMessagePresentTraceFailureIndication:
		return "TraceFailureIndication"
	case ngapType.InitiatingMessagePresentTraceStart:
		return "TraceStart"
	case ngapType.InitiatingMessagePresentUEContextReleaseRequest:
		return "UEContextReleaseRequest"
	case ngapType.InitiatingMessagePresentUERadioCapabilityInfoIndication:
		return "UERadioCapabilityInfoIndication"
	case ngapType.InitiatingMessagePresentUETNLABindingReleaseRequest:
		return "UETNLABindingReleaseRequest"
	case ngapType.InitiatingMessagePresentUplinkNASTransport:
		return "UplinkNASTransport"
	case ngapType.InitiatingMessagePresentUplinkNonUEAssociatedNRPPaTransport:
		return "UplinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentUplinkRANConfigurationTransfer:
		return "UplinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentUplinkRANStatusTransfer:
		return "UplinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentUplinkUEAssociatedNRPPaTransport:
		return "UplinkUEAssociatedNRPPaTransport"
	default:
		return "UnknownMessage"
	}
}

func getSuccessfulOutcomeMessageType(present int) string {
	switch present {
	case ngapType.SuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.SuccessfulOutcomePresentAMFConfigurationUpdateAcknowledge:
		return "AMFConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCancelAcknowledge:
		return "HandoverCancelAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCommand:
		return "HandoverCommand"
	case ngapType.SuccessfulOutcomePresentHandoverRequestAcknowledge:
		return "HandoverRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentInitialContextSetupResponse:
		return "InitialContextSetupResponse"
	case ngapType.SuccessfulOutcomePresentNGResetAcknowledge:
		return "NGResetAcknowledge"
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return "NGSetupResponse"
	case ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge:
		return "PathSwitchRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyResponse:
		return "PDUSessionResourceModifyResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyConfirm:
		return "PDUSessionResourceModifyConfirm"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse:
		return "PDUSessionResourceReleaseResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceSetupResponse:
		return "PDUSessionResourceSetupResponse"
	case ngapType.SuccessfulOutcomePresentPWSCancelResponse:
		return "PWSCancelResponse"
	case ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge:
		return "RANConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentUEContextModificationResponse:
		return "UEContextModificationResponse"
	case ngapType.SuccessfulOutcomePresentUEContextReleaseComplete:
		return "UEContextReleaseComplete"
	case ngapType.SuccessfulOutcomePresentUERadioCapabilityCheckResponse:
		return "UERadioCapabilityCheckResponse"
	case ngapType.SuccessfulOutcomePresentWriteReplaceWarningResponse:
		return "WriteReplaceWarningResponse"
	default:
		return "Unknown"
	}
}

func getUnsuccessfulOutcomeMessageType(present int) string {
	switch present {
	case ngapType.UnsuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.UnsuccessfulOutcomePresentAMFConfigurationUpdateFailure:
		return "AMFConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverPreparationFailure:
		return "HandoverPreparationFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverFailure:
		return "HandoverFailure"
	case ngapType.UnsuccessfulOutcomePresentInitialContextSetupFailure:
		return "InitialContextSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return "NGSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure:
		return "PathSwitchRequestFailure"
	case ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure:
		return "RANConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentUEContextModificationFailure:
		return "UEContextModificationFailure"
	default:
		return "Unknown"
	}
}

func NgapMsgHandler(conn *sctp.SCTPConn, ue *context.AmfUe, msg NgapMsg) {
	DispatchNgapMsg(conn, msg.Ran, msg.NgapMsg)
}

func DispatchNgapMsg(conn *sctp.SCTPConn, ran *context.AmfRan, pdu *ngapType.NGAPPDU) {
	messageType := getMessageType(pdu)

	peerAddr := conn.RemoteAddr()
	var peerAddrStr string
	if peerAddr != nil {
		peerAddrStr = peerAddr.String()
	} else {
		peerAddrStr = ""
	}
	spanName := fmt.Sprintf("AMF NGAP %s", messageType)
	ctx, span := tracer.Start(ctxt.Background(), spanName,
		trace.WithAttributes(
			attribute.String("net.peer", peerAddrStr),
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
		case ngapType.ProcedureCodeLocationReportingFailureIndication:
			HandleLocationReportingFailureIndication(ran, pdu)
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
		case ngapType.ProcedureCodeRRCInactiveTransitionReport:
			HandleRRCInactiveTransitionReport(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceNotify:
			HandlePDUSessionResourceNotify(ctx, ran, pdu)
		case ngapType.ProcedureCodePathSwitchRequest:
			HandlePathSwitchRequest(ctx, ran, pdu)
		case ngapType.ProcedureCodeLocationReport:
			HandleLocationReport(ctx, ran, pdu)
		case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
			HandleUplinkUEAssociatedNRPPATransport(ran, pdu)
		case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
			HandleUplinkRanConfigurationTransfer(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
			HandlePDUSessionResourceModifyIndication(ctx, ran, pdu)
		case ngapType.ProcedureCodeCellTrafficTrace:
			HandleCellTrafficTrace(ctx, ran, pdu)
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
			HandleUEContextReleaseComplete(ctx, ran, pdu)
		case ngapType.ProcedureCodePDUSessionResourceRelease:
			HandlePDUSessionResourceReleaseResponse(ctx, ran, pdu)
		case ngapType.ProcedureCodeUERadioCapabilityCheck:
			HandleUERadioCapabilityCheckResponse(ran, pdu)
		case ngapType.ProcedureCodeAMFConfigurationUpdate:
			HandleAMFconfigurationUpdateAcknowledge(ran, pdu)
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

	// Removing Stale Connections in AmfRanPool
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		amfRan := value.(*context.AmfRan)

		errorConn := sctp.NewSCTPConn(-1, nil)
		if reflect.DeepEqual(amfRan.Conn, errorConn) {
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
