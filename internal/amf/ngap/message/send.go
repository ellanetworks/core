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
	"github.com/free5gc/aper"
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

func SendToRanUe(ctx ctxt.Context, ue *context.RanUe, packet []byte, ngapMsgType NGAPProcedure) error {
	var ran *context.AmfRan

	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if ran = ue.Ran; ran == nil {
		return fmt.Errorf("ran is nil")
	}

	err := SendToRan(ctx, ran, packet, ngapMsgType)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func NasSendToRan(ctx ctxt.Context, ue *context.AmfUe, packet []byte, msgType NGAPProcedure) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	ranUe := ue.RanUe
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}

	err := SendToRanUe(ctx, ranUe, packet, msgType)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendDownlinkNasTransport(ctx ctxt.Context, ue *context.RanUe, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if len(nasPdu) == 0 {
		return fmt.Errorf("nas pdu is nil")
	}

	pkt, err := BuildDownlinkNasTransport(ue, nasPdu, mobilityRestrictionList)
	if err != nil {
		return fmt.Errorf("error building DownlinkNasTransport: %s", err.Error())
	}

	err = SendToRanUe(ctx, ue, pkt, NGAPProcedureDownlinkNasTransport)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendPDUSessionResourceReleaseCommand(ctx ctxt.Context, ue *context.RanUe, nasPdu []byte, pduSessionResourceReleasedList ngapType.PDUSessionResourceToReleaseListRelCmd) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	pkt, err := BuildPDUSessionResourceReleaseCommand(ue.AmfUeNgapID, ue.RanUeNgapID, nasPdu, pduSessionResourceReleasedList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource release: %s", err.Error())
	}

	err = SendToRanUe(ctx, ue, pkt, NGAPProcedurePDUSessionResourceReleaseCommand)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendUEContextReleaseCommand(ctx ctxt.Context, ue *context.RanUe, action context.RelAction, causePresent int, cause aper.Enumerated) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	pkt, err := BuildUEContextReleaseCommand(ue, causePresent, cause)
	if err != nil {
		return fmt.Errorf("error building ue context release: %s", err.Error())
	}

	ue.ReleaseAction = action

	err = SendToRanUe(ctx, ue, pkt, NGAPProcedureUEContextReleaseCommand)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendHandoverCancelAcknowledge(ctx ctxt.Context, ue *context.RanUe, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	ue.Log.Info("Send Handover Cancel Acknowledge")

	pkt, err := BuildHandoverCancelAcknowledge(ue, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building handover cancel acknowledge: %s", err.Error())
	}
	err = SendToRanUe(ctx, ue, pkt, NGAPProcedureHandoverCancelAcknowledge)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}
	return nil
}

func SendPDUSessionResourceSetupRequest(ctx ctxt.Context, ue *context.RanUe, nasPdu []byte, pduSessionResourceSetupRequestList ngapType.PDUSessionResourceSetupListSUReq) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if len(pduSessionResourceSetupRequestList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	pkt, err := BuildPDUSessionResourceSetupRequest(ue, nasPdu, pduSessionResourceSetupRequestList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource setup request: %s", err.Error())
	}

	err = SendToRanUe(ctx, ue, pkt, NGAPProcedurePDUSessionResourceSetupRequest)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendPDUSessionResourceModifyConfirm(
	ctx ctxt.Context,
	ue *context.RanUe,
	pduSessionResourceModifyConfirmList ngapType.PDUSessionResourceModifyListModCfm,
	pduSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if len(pduSessionResourceModifyConfirmList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	if len(pduSessionResourceFailedToModifyList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	pkt, err := BuildPDUSessionResourceModifyConfirm(ue, pduSessionResourceModifyConfirmList,
		pduSessionResourceFailedToModifyList, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building pdu session resource modify confirm: %s", err.Error())
	}

	err = SendToRanUe(ctx, ue, pkt, NGAPProcedurePDUSessionResourceModifyConfirm)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendInitialContextSetupRequest(
	ctx ctxt.Context,
	amfUe *context.AmfUe,
	nasPdu []byte,
	pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq,
	supportedGUAMI *models.Guami,
) error {
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	if pduSessionResourceSetupRequestList != nil {
		if len(pduSessionResourceSetupRequestList.List) > context.MaxNumOfPDUSessions {
			return fmt.Errorf("pdu list out of range")
		}
	}

	pkt, err := BuildInitialContextSetupRequest(ctx, amfUe, nasPdu, pduSessionResourceSetupRequestList, supportedGUAMI)
	if err != nil {
		return fmt.Errorf("error building initial context setup request: %s", err)
	}

	amfUe.RanUe.SentInitialContextSetupRequest = true

	err = NasSendToRan(ctx, amfUe, pkt, NGAPProcedureInitialContextSetupRequest)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

// pduSessionResourceHandoverList: provided by amf and transfer is return from smf
// pduSessionResourceToReleaseList: provided by amf and transfer is return from smf
// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
// when received node can't comprehend the IE or missing IE
func SendHandoverCommand(
	ctx ctxt.Context,
	sourceUe *context.RanUe,
	pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList,
	pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd,
	container ngapType.TargetToSourceTransparentContainer,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) error {
	if sourceUe == nil {
		return fmt.Errorf("source ue is nil")
	}

	if len(pduSessionResourceHandoverList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	if len(pduSessionResourceToReleaseList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	pkt, err := BuildHandoverCommand(sourceUe, pduSessionResourceHandoverList, pduSessionResourceToReleaseList,
		container, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building handover command: %s", err.Error())
	}

	err = SendToRanUe(ctx, sourceUe, pkt, NGAPProcedureHandoverCommand)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

// cause = initiate the Handover Cancel procedure with the appropriate value for the Cause IE.
// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
// when received node can't comprehend the IE or missing IE
func SendHandoverPreparationFailure(ctx ctxt.Context, sourceUe *context.RanUe, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	if sourceUe == nil {
		return fmt.Errorf("source ue is nil")
	}

	sourceUe.Log.Info("Send Handover Preparation Failure")

	amfUe := sourceUe.AmfUe
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	amfUe.SetOnGoing(&context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedureNothing,
	})

	pkt, err := BuildHandoverPreparationFailure(sourceUe, cause, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building handover preparation failure: %s", err.Error())
	}

	err = SendToRanUe(ctx, sourceUe, pkt, NGAPProcedureHandoverPreparationFailure)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

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
	if targetUeTmp, err := targetRan.NewRanUe(context.RanUeNgapIDUnspecified); err != nil {
		return fmt.Errorf("error creating target ue: %s", err.Error())
	} else {
		targetUe = targetUeTmp
	}

	err := context.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		return fmt.Errorf("attach source ue target ue error: %s", err.Error())
	}

	pkt, err := BuildHandoverRequest(ctx, targetUe, cause, pduSessionResourceSetupListHOReq, sourceToTargetTransparentContainer, supportedPLMN, supportedGUAMI)
	if err != nil {
		return fmt.Errorf("error building handover request: %s", err.Error())
	}

	err = SendToRanUe(ctx, targetUe, pkt, NGAPProcedureHandoverRequest)
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

	pkt, err := BuildPathSwitchRequestAcknowledge(ctx, ue, pduSessionResourceSwitchedList, pduSessionResourceReleasedList,
		newSecurityContextIndicator, coreNetworkAssistanceInformation, rrcInactiveTransitionReportRequest,
		criticalityDiagnostics, supportedPLMN)
	if err != nil {
		return fmt.Errorf("error building path switch request acknowledge: %s", err.Error())
	}

	err = SendToRanUe(ctx, ue, pkt, NGAPProcedurePathSwitchRequestAcknowledge)
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

// An AMF shall be able to instruct other peer CP NFs, subscribed to receive such a notification,
// that it will be unavailable on this AMF and its corresponding target AMF(s).
// If CP NF does not subscribe to receive AMF unavailable notification, the CP NF may attempt
// forwarding the transaction towards the old AMF and detect that the AMF is unavailable. When
// it detects unavailable, it marks the AMF and its associated GUAMI(s) as unavailable.
// Defined in 23.501 5.21.2.2.2
func SendAMFStatusIndication(ctx ctxt.Context, ran *context.AmfRan, unavailableGUAMIList ngapType.UnavailableGUAMIList) error {
	if ran == nil {
		return fmt.Errorf("ran is nil")
	}

	ran.Log.Info("Send AMF Status Indication")

	if len(unavailableGUAMIList.List) > context.MaxNumOfServedGuamiList {
		return fmt.Errorf("guami List out of range")
	}

	pkt, err := BuildAMFStatusIndication(unavailableGUAMIList)
	if err != nil {
		return fmt.Errorf("error building amf status indication: %s", err.Error())
	}

	err = SendToRan(ctx, ran, pkt, NGAPProcedureAMFStatusIndication)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

// AOI List is from SMF
// The SMF may subscribe to the UE mobility event notification from the AMF
// (e.g. location reporting, UE moving into or out of Area Of Interest) TS 23.502 4.3.2.2.1 Step.17
// The Location Reporting Control message shall identify the UE for which reports are requested and may include
// Reporting Type, Location Reporting Level, Area Of Interest and Request Reference ID
// TS 23.502 4.10 LocationReportingProcedure
// The AMF may request the NG-RAN location reporting with event reporting type (e.g. UE location or UE presence
// in Area of Interest), reporting mode and its related parameters (e.g. number of reporting) TS 23.501 5.4.7
// Location Reference ID To Be Cancelled IE shall be present if the Event Type IE is set to "Stop UE presence
// in the area of interest". otherwise set it to 0
func SendLocationReportingControl(
	ctx ctxt.Context,
	ue *context.RanUe,
	AOIList *ngapType.AreaOfInterestList,
	LocationReportingReferenceIDToBeCancelled int64,
	eventType ngapType.EventType,
) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if AOIList != nil && len(AOIList.List) > context.MaxNumOfAOI {
		return fmt.Errorf("aoi list out of range")
	}

	if eventType.Value == ngapType.EventTypePresentStopUePresenceInAreaOfInterest {
		if LocationReportingReferenceIDToBeCancelled < 1 || LocationReportingReferenceIDToBeCancelled > 64 {
			return fmt.Errorf("location reporting reference id to be cancelled out of range (should be 1 ~ 64)")
		}
	}

	pkt, err := BuildLocationReportingControl(ue, AOIList, LocationReportingReferenceIDToBeCancelled, eventType)
	if err != nil {
		return fmt.Errorf("error building location reporting control: %s", err.Error())
	}

	err = SendToRanUe(ctx, ue, pkt, NGAPProcedureLocationReportingControl)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func nativeToNetworkEndianness32(value uint32) uint32 {
	var b [4]byte
	binary.NativeEndian.PutUint32(b[:], value)
	return binary.BigEndian.Uint32(b[:])
}
