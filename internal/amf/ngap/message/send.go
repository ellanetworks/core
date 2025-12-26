// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	ctxt "context"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/ngap/message")

type NGAPProcedure string

const (
	// Non-UE associated NGAP procedures
	NGAPProcedureNGSetupResponse                   NGAPProcedure = "NGSetupResponse"
	NGAPProcedureNGSetupFailure                    NGAPProcedure = "NGSetupFailure"
	NGAPProcedurePaging                            NGAPProcedure = "Paging"
	NGAPProcedureNGResetAcknowledge                NGAPProcedure = "NGResetAcknowledge"
	NGAPProcedureErrorIndication                   NGAPProcedure = "ErrorIndication"
	NGAPProcedureRanConfigurationUpdateAcknowledge NGAPProcedure = "RANConfigurationUpdateAcknowledge"
	NGAPProcedureRanConfigurationUpdateFailure     NGAPProcedure = "RANConfigurationUpdateFailure"
	NGAPProcedureAMFStatusIndication               NGAPProcedure = "AMFStatusIndication"
	NGAPProcedureDownlinkRanConfigurationTransfer  NGAPProcedure = "DownlinkRANConfigurationTransfer"

	// UE-associated NGAP procedures
	NGAPProcedureInitialContextSetupRequest       NGAPProcedure = "InitialContextSetupRequest"
	NGAPProcedurePDUSessionResourceModifyRequest  NGAPProcedure = "PDUSessionResourceModifyRequest"
	NGAPProcedurePDUSessionResourceModifyConfirm  NGAPProcedure = "PDUSessionResourceModifyConfirm"
	NGAPProcedurePDUSessionResourceSetupRequest   NGAPProcedure = "PDUSessionResourceSetupRequest"
	NGAPProcedurePDUSessionResourceReleaseCommand NGAPProcedure = "PDUSessionResourceReleaseCommand"
	NGAPProcedureDownlinkNasTransport             NGAPProcedure = "DownlinkNasTransport"
	NGAPProcedureLocationReportingControl         NGAPProcedure = "LocationReportingControl"
	NGAPProcedurePathSwitchRequestFailure         NGAPProcedure = "PathSwitchRequestFailure"
	NGAPProcedurePathSwitchRequestAcknowledge     NGAPProcedure = "PathSwitchRequestAcknowledge"
	NGAPProcedureHandoverRequest                  NGAPProcedure = "HandoverRequest"
	NGAPProcedureHandoverCommand                  NGAPProcedure = "HandoverCommand"
	NGAPProcedureHandoverCancelAcknowledge        NGAPProcedure = "HandoverCancelAcknowledge"
	NGAPProcedureHandoverPreparationFailure       NGAPProcedure = "HandoverPreparationFailure"
	NGAPProcedureUEContextReleaseCommand          NGAPProcedure = "UEContextReleaseCommand"
)

func getSCTPStreamID(msgType NGAPProcedure) (uint16, error) {
	switch msgType {
	// Non-UE procedures
	case NGAPProcedureNGSetupResponse, NGAPProcedureNGSetupFailure,
		NGAPProcedurePaging, NGAPProcedureNGResetAcknowledge,
		NGAPProcedureErrorIndication, NGAPProcedureRanConfigurationUpdateAcknowledge,
		NGAPProcedureRanConfigurationUpdateFailure, NGAPProcedureAMFStatusIndication,
		NGAPProcedureDownlinkRanConfigurationTransfer:
		return 0, nil

	// UE-associated procedures
	case NGAPProcedureInitialContextSetupRequest, NGAPProcedureUEContextReleaseCommand,
		NGAPProcedureDownlinkNasTransport, NGAPProcedurePDUSessionResourceSetupRequest,
		NGAPProcedurePDUSessionResourceReleaseCommand, NGAPProcedureHandoverRequest,
		NGAPProcedureHandoverCommand, NGAPProcedureHandoverPreparationFailure,
		NGAPProcedurePathSwitchRequestAcknowledge, NGAPProcedurePDUSessionResourceModifyRequest,
		NGAPProcedurePDUSessionResourceModifyConfirm, NGAPProcedureHandoverCancelAcknowledge,
		NGAPProcedureLocationReportingControl, NGAPProcedurePathSwitchRequestFailure:
		return 1, nil
	default:
		return 0, fmt.Errorf("NGAP message type (%s) not supported", msgType)
	}
}

// TO delete once RealNGAPSender.SendToRan is used everywhere
func SendToRan(ctx ctxt.Context, ran *context.AmfRan, packet []byte, msgType NGAPProcedure) error {
	ctx, span := tracer.Start(ctx, "Send To RAN",
		trace.WithAttributes(
			attribute.String("ngap.messageType", string(msgType)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	sid, err := getSCTPStreamID(msgType)
	if err != nil {
		return fmt.Errorf("could not determine SCTP stream ID from NGAP message type (%s): %s", msgType, err.Error())
	}

	defer func() {
		err := recover()
		if err != nil {
			logger.AmfLog.Warn("could not send to ran", zap.Any("error", err))
		}
	}()

	if ran == nil {
		return fmt.Errorf("ran is nil")
	}

	if len(packet) == 0 {
		return fmt.Errorf("packet len is 0")
	}

	if ran.Conn == nil {
		return fmt.Errorf("ran conn is nil")
	}

	if ran.Conn.RemoteAddr() == nil {
		return fmt.Errorf("ran address is nil")
	}

	info := sctp.SndRcvInfo{
		Stream: sid,
		PPID:   nativeToNetworkEndianness32(sctp.NGAPPPID),
	}
	if _, err := ran.Conn.SCTPWrite(packet, &info); err != nil {
		return fmt.Errorf("send write to sctp connection: %s", err.Error())
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		string(msgType),
		logger.DirectionOutbound,
		ran.Conn.LocalAddr().String(),
		ran.Conn.RemoteAddr().String(),
		packet,
	)

	return nil
}

/*The PGW-C+SMF (V-SMF in the case of home-routed roaming scenario only) sends
a Nsmf_PDUSession_CreateSMContext Response(N2 SM Information (PDU Session ID, cause code)) to the AMF.*/
// Cause is from SMF
// pduSessionResourceSetupList provided by AMF, and the transfer data is from SMF
// sourceToTargetTransparentContainer is received from S-RAN
// nsci: new security context indicator, if amfUe has updated security context, set nsci to true, otherwise set to false
// N2 handover in same AMF
func SendHandoverRequest(
	ctx ctxt.Context,
	sourceUe *context.RanUe,
	targetRan *context.AmfRan,
	cause ngapType.Cause,
	pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq,
	sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer,
	supportedPLMN *models.PlmnSupportItem,
	supportedGUAMI *models.Guami,
) error {
	if sourceUe == nil {
		return fmt.Errorf("source ue is nil")
	}

	amfUe := sourceUe.AmfUe
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}
	if targetRan == nil {
		return fmt.Errorf("target ran is nil")
	}

	if sourceUe.TargetUe != nil {
		return fmt.Errorf("handover required duplicated")
	}

	if len(pduSessionResourceSetupListHOReq.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	if len(sourceToTargetTransparentContainer.Value) == 0 {
		return fmt.Errorf("source to target transparent container is nil")
	}

	var targetUe *context.RanUe
	if targetUeTmp, err := targetRan.NewRanUe(models.RanUeNgapIDUnspecified); err != nil {
		return fmt.Errorf("error creating target ue: %s", err.Error())
	} else {
		targetUe = targetUeTmp
	}

	err := context.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		return fmt.Errorf("attach source ue target ue error: %s", err.Error())
	}

	pkt, err := BuildHandoverRequest(
		targetUe.AmfUeNgapID,
		targetUe.HandOverType,
		targetUe.AmfUe.Ambr.Uplink,
		targetUe.AmfUe.Ambr.Downlink,
		targetUe.AmfUe.UESecurityCapability,
		targetUe.AmfUe.NCC,
		targetUe.AmfUe.NH,
		cause,
		pduSessionResourceSetupListHOReq,
		sourceToTargetTransparentContainer,
		supportedPLMN,
		supportedGUAMI,
	)
	if err != nil {
		return fmt.Errorf("error building handover request: %s", err.Error())
	}

	err = SendToRan(ctx, targetUe.Ran, pkt, NGAPProcedureHandoverRequest)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

// pduSessionResourceSwitchedList: provided by AMF, and the transfer data is from SMF
// pduSessionResourceReleasedList: provided by AMF, and the transfer data is from SMF
// newSecurityContextIndicator: if AMF has activated a new 5G NAS security context, set it to true,
// otherwise set to false
// coreNetworkAssistanceInformation: provided by AMF, based on collection of UE behaviour statistics
// and/or other available
// information about the expected UE behaviour. TS 23.501 5.4.6, 5.4.6.2
// rrcInactiveTransitionReportRequest: configured by amf
// criticalityDiagnostics: from received node when received not comprehended IE or missing IE
func SendPathSwitchRequestAcknowledge(
	ctx ctxt.Context,
	ue *context.RanUe,
	pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList,
	pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck,
	newSecurityContextIndicator bool,
	coreNetworkAssistanceInformation *ngapType.CoreNetworkAssistanceInformation,
	rrcInactiveTransitionReportRequest *ngapType.RRCInactiveTransitionReportRequest,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
	supportedPLMN *models.PlmnSupportItem,
) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if len(pduSessionResourceSwitchedList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	if len(pduSessionResourceReleasedList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	pkt, err := BuildPathSwitchRequestAcknowledge(
		ue.AmfUeNgapID,
		ue.RanUeNgapID,
		ue.AmfUe.UESecurityCapability,
		ue.AmfUe.NCC,
		ue.AmfUe.NH,
		pduSessionResourceSwitchedList,
		pduSessionResourceReleasedList,
		newSecurityContextIndicator,
		coreNetworkAssistanceInformation,
		rrcInactiveTransitionReportRequest,
		criticalityDiagnostics,
		supportedPLMN,
	)
	if err != nil {
		return fmt.Errorf("error building path switch request acknowledge: %s", err.Error())
	}

	err = SendToRan(ctx, ue.Ran, pkt, NGAPProcedurePathSwitchRequestAcknowledge)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendPaging(ctx ctxt.Context, ue *context.AmfUe, ngapBuf []byte) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	amfSelf := context.AMFSelf()

	amfSelf.Mutex.Lock()
	defer amfSelf.Mutex.Unlock()

	taiList := ue.RegistrationArea

	for _, ran := range amfSelf.AmfRanPool {
		for _, item := range ran.SupportedTAList {
			if context.InTaiList(item.Tai, taiList) {
				err := SendToRan(ctx, ran, ngapBuf, NGAPProcedurePaging)
				if err != nil {
					ue.Log.Error("failed to send paging", zap.Error(err))
					continue
				}
				ue.Log.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))
				break
			}
		}
	}

	if amfSelf.T3513Cfg.Enable {
		cfg := amfSelf.T3513Cfg
		ue.T3513 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			ue.Log.Warn("t3513 expires, retransmit paging", zap.Int32("retry", expireTimes))
			for _, ran := range amfSelf.AmfRanPool {
				for _, item := range ran.SupportedTAList {
					if context.InTaiList(item.Tai, taiList) {
						err := SendToRan(ctx, ran, ngapBuf, NGAPProcedurePaging)
						if err != nil {
							ue.Log.Error("failed to send paging", zap.Error(err))
							continue
						}
						ue.Log.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))
						break
					}
				}
			}
		}, func() {
			ue.Log.Warn("T3513 expires, abort paging procedure", zap.Int32("retry", cfg.MaxRetryTimes))
			ue.T3513 = nil // clear the timer
		})
	}

	return nil
}

func nativeToNetworkEndianness32(value uint32) uint32 {
	var b [4]byte
	binary.NativeEndian.PutUint32(b[:], value)
	return binary.BigEndian.Uint32(b[:])
}
