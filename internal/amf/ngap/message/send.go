// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

func SendToRan(ran *context.AmfRan, packet []byte) error {
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

	if _, err := ran.Conn.Write(packet); err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func SendToRanUe(ue *context.RanUe, packet []byte) error {
	var ran *context.AmfRan

	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if ran = ue.Ran; ran == nil {
		return fmt.Errorf("ran is nil")
	}

	err := SendToRan(ran, packet)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func NasSendToRan(ue *context.AmfUe, accessType models.AccessType, packet []byte) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}
	ranUe := ue.RanUe[accessType]
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}
	err := SendToRanUe(ranUe, packet)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}
	return nil
}

func SendNGSetupResponse(ctx ctxt.Context, ran *context.AmfRan) error {
	pkt, err := BuildNGSetupResponse(ctx)
	if err != nil {
		return fmt.Errorf("error building NG Setup Response: %s", err.Error())
	}
	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioNGSetupResponse,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
	)

	return nil
}

func SendNGSetupFailure(ran *context.AmfRan, cause ngapType.Cause) error {
	if cause.Present == ngapType.CausePresentNothing {
		return fmt.Errorf("cause present is nil")
	}

	pkt, err := BuildNGSetupFailure(cause)
	if err != nil {
		return fmt.Errorf("error building NG Setup Failure: %s", err.Error())
	}

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioNGSetupFailure,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
		zap.String("cause", causeToString(&cause)),
	)

	return nil
}

func SendNGResetAcknowledge(ran *context.AmfRan, partOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList) error {
	if partOfNGInterface != nil && len(partOfNGInterface.List) == 0 {
		return fmt.Errorf("length of partOfNGInterface is 0")
	}

	pkt, err := BuildNGResetAcknowledge(partOfNGInterface)
	if err != nil {
		return fmt.Errorf("error building NG Reset Acknowledge: %s", err.Error())
	}

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioNGResetAcknowledge,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
	)

	return nil
}

func SendDownlinkNasTransport(ue *context.RanUe, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) error {
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

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioDownlinkNasTransport,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
	)

	return nil
}

func SendPDUSessionResourceReleaseCommand(ue *context.RanUe, nasPdu []byte, pduSessionResourceReleasedList ngapType.PDUSessionResourceToReleaseListRelCmd) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	pkt, err := BuildPDUSessionResourceReleaseCommand(ue, nasPdu, pduSessionResourceReleasedList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource release: %s", err.Error())
	}

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioPDUSessionResourceReleaseCommand,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
	)

	return nil
}

func SendUEContextReleaseCommand(ue *context.RanUe, action context.RelAction, causePresent int, cause aper.Enumerated) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	pkt, err := BuildUEContextReleaseCommand(ue, causePresent, cause)
	if err != nil {
		return fmt.Errorf("error building ue context release: %s", err.Error())
	}

	ue.ReleaseAction = action
	if ue.AmfUe != nil && ue.Ran != nil {
		ue.AmfUe.ReleaseCause[ue.Ran.AnType] = &context.CauseAll{
			NgapCause: &models.NgApCause{
				Group: int32(causePresent),
				Value: int32(cause),
			},
		}
	}

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioUEContextReleaseCommand,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
		zap.String("cause", causePresentToString(causePresent, cause)),
	)

	return nil
}

func SendErrorIndication(ran *context.AmfRan, amfUeNgapID, ranUeNgapID *int64, cause *ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	if ran == nil {
		return fmt.Errorf("ran is nil")
	}

	pkt, err := BuildErrorIndication(amfUeNgapID, ranUeNgapID, cause, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building error indication: %s", err.Error())
	}

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioErrorIndication,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
		zap.String("cause", causeToString(cause)),
	)

	return nil
}

func SendHandoverCancelAcknowledge(ue *context.RanUe, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	ue.Log.Info("Send Handover Cancel Acknowledge")

	pkt, err := BuildHandoverCancelAcknowledge(ue, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building handover cancel acknowledge: %s", err.Error())
	}
	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}
	return nil
}

func SendPDUSessionResourceSetupRequest(ue *context.RanUe, nasPdu []byte, pduSessionResourceSetupRequestList ngapType.PDUSessionResourceSetupListSUReq) error {
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

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioPDUSessionResourceSetupRequest,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
	)

	return nil
}

func SendPDUSessionResourceModifyConfirm(
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

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioPDUSessionResourceModifyConfirm,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
	)

	return nil
}

// pduSessionResourceModifyRequestList: from SMF
func SendPDUSessionResourceModifyRequest(ue *context.RanUe, pduSessionResourceModifyRequestList ngapType.PDUSessionResourceModifyListModReq) error {
	if ue == nil {
		return fmt.Errorf("ran ue is nil")
	}

	ue.Log.Info("Send PDU Session Resource Modify Request")

	if len(pduSessionResourceModifyRequestList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	pkt, err := BuildPDUSessionResourceModifyRequest(ue, pduSessionResourceModifyRequestList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource modify request: %s", err.Error())
	}

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioPDUSessionResourceModifyRequest,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
	)

	return nil
}

func SendInitialContextSetupRequest(
	ctx ctxt.Context,
	amfUe *context.AmfUe,
	anType models.AccessType,
	nasPdu []byte,
	pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq,
	rrcInactiveTransitionReportRequest *ngapType.RRCInactiveTransitionReportRequest,
	coreNetworkAssistanceInfo *ngapType.CoreNetworkAssistanceInformation,
	emergencyFallbackIndicator *ngapType.EmergencyFallbackIndicator,
) error {
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	if pduSessionResourceSetupRequestList != nil {
		if len(pduSessionResourceSetupRequestList.List) > context.MaxNumOfPDUSessions {
			return fmt.Errorf("pdu list out of range")
		}
	}

	pkt, err := BuildInitialContextSetupRequest(ctx, amfUe, anType, nasPdu, pduSessionResourceSetupRequestList,
		rrcInactiveTransitionReportRequest, coreNetworkAssistanceInfo, emergencyFallbackIndicator)
	if err != nil {
		return fmt.Errorf("error building initial context setup request: %s", err.Error())
	}

	amfUe.RanUe[anType].SentInitialContextSetupRequest = true

	err = NasSendToRan(amfUe, anType, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioInitialContextSetupRequest,
		logger.DirectionOutbound,
		pkt,
		amfUe.RanUe[anType].Ran.GnbID,
		zap.String("ranName", amfUe.RanUe[anType].Ran.Name),
		zap.String("ranID", amfUe.RanUe[anType].Ran.GnbID),
		zap.String("ranIP", amfUe.RanUe[anType].Ran.GnbIP),
		zap.String("supi", amfUe.Supi),
		zap.String("anType", string(anType)),
		zap.String("nasPdu", fmt.Sprintf("%x", nasPdu)),
	)

	return nil
}

// pduSessionResourceHandoverList: provided by amf and transfer is return from smf
// pduSessionResourceToReleaseList: provided by amf and transfer is return from smf
// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
// when received node can't comprehend the IE or missing IE
func SendHandoverCommand(
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

	err = SendToRanUe(sourceUe, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioHandoverCommand,
		logger.DirectionOutbound,
		pkt,
		sourceUe.Ran.GnbID,
		zap.String("ranName", sourceUe.Ran.Name),
		zap.String("ranID", sourceUe.Ran.GnbID),
		zap.String("ranIP", sourceUe.Ran.GnbIP),
	)

	return nil
}

// cause = initiate the Handover Cancel procedure with the appropriate value for the Cause IE.
// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
// when received node can't comprehend the IE or missing IE
func SendHandoverPreparationFailure(sourceUe *context.RanUe, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	if sourceUe == nil {
		return fmt.Errorf("source ue is nil")
	}

	sourceUe.Log.Info("Send Handover Preparation Failure")

	amfUe := sourceUe.AmfUe
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	amfUe.SetOnGoing(sourceUe.Ran.AnType, &context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedureNothing,
	})

	pkt, err := BuildHandoverPreparationFailure(sourceUe, cause, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building handover preparation failure: %s", err.Error())
	}

	err = SendToRanUe(sourceUe, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioHandoverPreparationFailure,
		logger.DirectionOutbound,
		pkt,
		sourceUe.Ran.GnbID,
		zap.String("ranName", sourceUe.Ran.Name),
		zap.String("ranID", sourceUe.Ran.GnbID),
		zap.String("ranIP", sourceUe.Ran.GnbIP),
		zap.String("cause", causeToString(&cause)),
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
func SendHandoverRequest(ctx ctxt.Context, sourceUe *context.RanUe, targetRan *context.AmfRan, cause ngapType.Cause, pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq, sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer) error {
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

	pkt, err := BuildHandoverRequest(ctx, targetUe, cause, pduSessionResourceSetupListHOReq, sourceToTargetTransparentContainer)
	if err != nil {
		return fmt.Errorf("error building handover request: %s", err.Error())
	}

	err = SendToRanUe(targetUe, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioHandoverRequest,
		logger.DirectionOutbound,
		pkt,
		targetRan.GnbID,
		zap.String("ranName", targetRan.Name),
		zap.String("ranID", targetRan.GnbID),
		zap.String("ranIP", targetRan.GnbIP),
		zap.String("supi", amfUe.Supi),
		zap.String("sourceRan", sourceUe.Ran.Name),
		zap.String("sourceRanID", sourceUe.Ran.GnbID),
		zap.String("sourceRanIP", sourceUe.Ran.GnbIP),
	)

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
		criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building path switch request acknowledge: %s", err.Error())
	}

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioPathSwitchRequestAcknowledge,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
	)

	return nil
}

// pduSessionResourceReleasedList: provided by AMF, and the transfer data is from SMF
// criticalityDiagnostics: from received node when received not comprehended IE or missing IE
func SendPathSwitchRequestFailure(
	ran *context.AmfRan,
	amfUeNgapID,
	ranUeNgapID int64,
	pduSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListPSFail,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) error {
	if pduSessionResourceReleasedList != nil && len(pduSessionResourceReleasedList.List) > context.MaxNumOfPDUSessions {
		return fmt.Errorf("pdu list out of range")
	}

	pkt, err := BuildPathSwitchRequestFailure(amfUeNgapID, ranUeNgapID, pduSessionResourceReleasedList,
		criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building path switch request failure: %s", err.Error())
	}

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioPathSwitchRequestFailure,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
	)

	return nil
}

// anType indicate amfUe send this msg for which accessType
// Paging Priority: is included only if the AMF receives an Namf_Communication_N1N2MessageTransfer message
// with an ARP value associated with
// priority services (e.g., MPS, MCS), as configured by the operator. (TS 23.502 4.2.3.3, TS 23.501 5.22.3)
// pagingOriginNon3GPP: TS 23.502 4.2.3.3 step 4b: If the UE is simultaneously registered over 3GPP and non-3GPP
// accesses in the same PLMN,
// the UE is in CM-IDLE state in both 3GPP access and non-3GPP access, and the PDU Session ID in step 3a
// is associated with non-3GPP access, the AMF sends a Paging message with associated access "non-3GPP" to
// NG-RAN node(s) via 3GPP access.
// more paging policy with 3gpp/non-3gpp access is described in TS 23.501 5.6.8
func SendPaging(ue *context.AmfUe, ngapBuf []byte) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	taiList := ue.RegistrationArea[models.AccessType3GPPAccess]
	context.AMFSelf().AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*context.AmfRan)
		for _, item := range ran.SupportedTAList {
			if context.InTaiList(item.Tai, taiList) {
				err := SendToRan(ran, ngapBuf)
				if err != nil {
					ue.GmmLog.Error("failed to send paging", zap.Error(err))
					continue
				}
				ue.GmmLog.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))
				break
			}
		}
		return true
	})

	if context.AMFSelf().T3513Cfg.Enable {
		cfg := context.AMFSelf().T3513Cfg
		ue.T3513 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			ue.GmmLog.Warn("t3513 expires, retransmit paging", zap.Int32("retry", expireTimes))
			context.AMFSelf().AmfRanPool.Range(func(key, value interface{}) bool {
				ran := value.(*context.AmfRan)
				for _, item := range ran.SupportedTAList {
					if context.InTaiList(item.Tai, taiList) {
						err := SendToRan(ran, ngapBuf)
						if err != nil {
							ue.GmmLog.Error("failed to send paging", zap.Error(err))
							continue
						}
						ue.GmmLog.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))
						break
					}
				}
				return true
			})
		}, func() {
			ue.GmmLog.Warn("T3513 expires, abort paging procedure", zap.Int32("retry", cfg.MaxRetryTimes))
			ue.T3513 = nil // clear the timer
		})
	}

	return nil
}

// criticality ->from received node when received node can't comprehend the IE or missing IE
func SendRanConfigurationUpdateAcknowledge(ran *context.AmfRan, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	if ran == nil {
		return fmt.Errorf("ran is nil")
	}

	pkt, err := BuildRanConfigurationUpdateAcknowledge(criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building ran configuration update acknowledge: %s", err.Error())
	}

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioRanConfigurationUpdateAcknowledge,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
	)

	return nil
}

// criticality ->from received node when received node can't comprehend the IE or missing IE
// If the AMF cannot accept the update,
// it shall respond with a RAN CONFIGURATION UPDATE FAILURE message and appropriate cause value.
func SendRanConfigurationUpdateFailure(ran *context.AmfRan, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	if ran == nil {
		return fmt.Errorf("ran is nil")
	}

	pkt, err := BuildRanConfigurationUpdateFailure(cause, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building ran configuration update failure: %s", err.Error())
	}

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioRanConfigurationUpdateFailure,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
		zap.String("cause", causeToString(&cause)),
	)

	return nil
}

// An AMF shall be able to instruct other peer CP NFs, subscribed to receive such a notification,
// that it will be unavailable on this AMF and its corresponding target AMF(s).
// If CP NF does not subscribe to receive AMF unavailable notification, the CP NF may attempt
// forwarding the transaction towards the old AMF and detect that the AMF is unavailable. When
// it detects unavailable, it marks the AMF and its associated GUAMI(s) as unavailable.
// Defined in 23.501 5.21.2.2.2
func SendAMFStatusIndication(ran *context.AmfRan, unavailableGUAMIList ngapType.UnavailableGUAMIList) error {
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

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioAMFStatusIndication,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
	)

	return nil
}

// SONConfigurationTransfer = sONConfigurationTransfer from uplink Ran Configuration Transfer
func SendDownlinkRanConfigurationTransfer(ran *context.AmfRan, transfer *ngapType.SONConfigurationTransfer) error {
	if ran == nil {
		return fmt.Errorf("ran is nil")
	}

	pkt, err := BuildDownlinkRanConfigurationTransfer(transfer)
	if err != nil {
		return fmt.Errorf("error building downlink ran configuration transfer: %s", err.Error())
	}

	err = SendToRan(ran, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioDownlinkRanConfigurationTransfer,
		logger.DirectionOutbound,
		pkt,
		ran.GnbID,
		zap.String("ranName", ran.Name),
		zap.String("ranID", ran.GnbID),
		zap.String("ranIP", ran.GnbIP),
	)

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

	err = SendToRanUe(ue, pkt)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	logger.LogRadioEvent(
		logger.RadioLocationReportingControl,
		logger.DirectionOutbound,
		pkt,
		ue.Ran.GnbID,
		zap.String("ranName", ue.Ran.Name),
		zap.String("ranID", ue.Ran.GnbID),
		zap.String("ranIP", ue.Ran.GnbIP),
	)

	return nil
}
