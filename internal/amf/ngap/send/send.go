// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package send

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// NGAPPPID is the SCTP payload protocol identifier for NGAP (TS 38.412 §7),
// set on every NGAP datagram the AMF sends and required on the listener.
const NGAPPPID uint32 = 60

var tracer = otel.Tracer("ella-core/amf/ngap/send")

type RealNGAPSender struct {
	Conn      *sctp.SCTPConn
	RadioName string
}

func (s *RealNGAPSender) SendToRan(ctx context.Context, packet []byte, msgType NGAPProcedure) error {
	ctx, span := tracer.Start(ctx, "ngap/send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("ngap.message_type", string(msgType)),
			attribute.Int("ngap.message_size", len(packet)),
			attribute.String("network.protocol.name", "ngap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	sid, err := getSCTPStreamID(msgType)
	if err != nil {
		return fmt.Errorf("could not determine SCTP stream ID from NGAP message type (%s): %s", msgType, err.Error())
	}

	if len(packet) == 0 {
		return fmt.Errorf("packet len is 0")
	}

	if s.Conn == nil {
		return fmt.Errorf("ran conn is nil")
	}

	localAddr := s.Conn.LocalAddr()
	if localAddr == nil {
		return fmt.Errorf("ran local address is nil")
	}

	remoteAddr := s.Conn.RemoteAddr()
	if remoteAddr == nil {
		return fmt.Errorf("ran remote address is nil")
	}

	info := sctp.SndRcvInfo{
		Stream: sid,
		PPID:   sctp.PPIDWireOrder(NGAPPPID),
	}
	if _, err := s.Conn.WriteMsg(packet, &info); err != nil {
		return fmt.Errorf("send write to sctp connection: %s", err.Error())
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		string(msgType),
		logger.DirectionOutbound,
		localAddr.String(),
		remoteAddr.String(),
		s.RadioName,
		packet,
	)

	return nil
}

func (s *RealNGAPSender) SendNGSetupResponse(ctx context.Context, guami *models.Guami, snssaiList []models.Snssai, amfName string, amfRelativeCapacity int64) error {
	pkt, err := buildNGSetupResponse(guami, snssaiList, amfName, amfRelativeCapacity)
	if err != nil {
		return fmt.Errorf("error building NG Setup Response: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureNGSetupResponse)
	if err != nil {
		return fmt.Errorf("couldn't send packet to ran: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendNGSetupFailure(ctx context.Context, cause *ngapType.Cause) error {
	pkt, err := buildNGSetupFailure(cause)
	if err != nil {
		return fmt.Errorf("error building NG Setup Failure: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureNGSetupFailure)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendNGResetAcknowledge(ctx context.Context, partOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList) error {
	pkt, err := buildNGResetAcknowledge(partOfNGInterface)
	if err != nil {
		return fmt.Errorf("error building NG Reset Acknowledge: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureNGResetAcknowledge)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendErrorIndication(ctx context.Context, amfUeNgapID, ranUeNgapID *int64, cause *ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	pkt, err := buildErrorIndication(amfUeNgapID, ranUeNgapID, cause, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building error indication: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureErrorIndication)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendRanConfigurationUpdateAcknowledge(ctx context.Context, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	pkt, err := buildRanConfigurationUpdateAcknowledge(criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building ran configuration update acknowledge: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureRanConfigurationUpdateAcknowledge)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendRanConfigurationUpdateFailure(ctx context.Context, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	pkt, err := buildRanConfigurationUpdateFailure(cause, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building ran configuration update failure: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureRanConfigurationUpdateFailure)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendDownlinkRanConfigurationTransfer(ctx context.Context, transfer *ngapType.SONConfigurationTransfer) error {
	pkt, err := buildDownlinkRanConfigurationTransfer(transfer)
	if err != nil {
		return fmt.Errorf("error building downlink ran configuration transfer: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureDownlinkRanConfigurationTransfer)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendPathSwitchRequestFailure(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, pduSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListPSFail, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	pkt, err := buildPathSwitchRequestFailure(amfUeNgapID, ranUeNgapID, pduSessionResourceReleasedList, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building path switch request failure: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedurePathSwitchRequestFailure)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

// Notifies peer CP NFs that this AMF and its GUAMI(s) are unavailable (TS 23.501 §5.21.2.2.2).
func (s *RealNGAPSender) SendAMFStatusIndication(ctx context.Context, unavailableGUAMIList ngapType.UnavailableGUAMIList) error {
	pkt, err := buildAMFStatusIndication(unavailableGUAMIList)
	if err != nil {
		return fmt.Errorf("error building amf status indication: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureAMFStatusIndication)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendUEContextReleaseCommand(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, causePresent int, cause aper.Enumerated) error {
	pkt, err := buildUEContextReleaseCommand(amfUeNgapID, ranUeNgapID, causePresent, cause)
	if err != nil {
		return fmt.Errorf("error building ue context release: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureUEContextReleaseCommand)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendDownlinkNasTransport(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) error {
	pkt, err := buildDownlinkNasTransport(amfUeNgapID, ranUeNgapID, nasPdu, mobilityRestrictionList)
	if err != nil {
		return fmt.Errorf("error building DownlinkNasTransport: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureDownlinkNasTransport)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendPDUSessionResourceReleaseCommand(ctx context.Context, amfUENgapID int64, ranUENgapID int64, nasPdu []byte, pduSessionResourceReleasedList ngapType.PDUSessionResourceToReleaseListRelCmd) error {
	pkt, err := buildPDUSessionResourceReleaseCommand(amfUENgapID, ranUENgapID, nasPdu, pduSessionResourceReleasedList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource release: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedurePDUSessionResourceReleaseCommand)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendHandoverCancelAcknowledge(ctx context.Context, amfUENgapID int64, ranUENgapID int64) error {
	pkt, err := buildHandoverCancelAcknowledge(amfUENgapID, ranUENgapID)
	if err != nil {
		return fmt.Errorf("error building handover cancel acknowledge: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureHandoverCancelAcknowledge)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendPDUSessionResourceModifyConfirm(ctx context.Context, amfUENgapID int64, ranUENgapID int64, pduSessionResourceModifyConfirmList ngapType.PDUSessionResourceModifyListModCfm, pduSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm) error {
	pkt, err := buildPDUSessionResourceModifyConfirm(amfUENgapID, ranUENgapID, pduSessionResourceModifyConfirmList, pduSessionResourceFailedToModifyList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource modify confirm: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedurePDUSessionResourceModifyConfirm)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendPDUSessionResourceModifyRequest(ctx context.Context, amfUENgapID int64, ranUENgapID int64, pduSessionResourceModifyList ngapType.PDUSessionResourceModifyListModReq) error {
	pkt, err := buildPDUSessionResourceModifyRequest(amfUENgapID, ranUENgapID, pduSessionResourceModifyList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource modify request: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedurePDUSessionResourceModifyRequest)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendPDUSessionResourceSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, nasPdu []byte, pduSessionResourceSetupRequestList ngapType.PDUSessionResourceSetupListSUReq) error {
	pkt, err := buildPDUSessionResourceSetupRequest(amfUeNgapID, ranUeNgapID, ambrUplink, ambrDownlink, nasPdu, pduSessionResourceSetupRequestList)
	if err != nil {
		return fmt.Errorf("error building pdu session resource setup request: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedurePDUSessionResourceSetupRequest)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendHandoverPreparationFailure(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	pkt, err := buildHandoverPreparationFailure(amfUeNgapID, ranUeNgapID, cause, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building handover preparation failure: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureHandoverPreparationFailure)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

// SMF-requested NG-RAN location reporting (TS 23.502 §4.10, TS 23.501 §5.4.7).
func (s *RealNGAPSender) SendLocationReportingControl(ctx context.Context, amfUENgapID int64, ranUENgapID int64, eventType ngapType.EventType) error {
	pkt, err := buildLocationReportingControl(amfUENgapID, ranUENgapID, eventType)
	if err != nil {
		return fmt.Errorf("error building location reporting control: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureLocationReportingControl)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendHandoverCommand(
	ctx context.Context,
	amfUeNgapID int64,
	ranUeNgapID int64,
	handOverType ngapType.HandoverType,
	pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList,
	pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd,
	container ngapType.TargetToSourceTransparentContainer,
) error {
	pkt, err := buildHandoverCommand(
		amfUeNgapID,
		ranUeNgapID,
		handOverType,
		pduSessionResourceHandoverList,
		pduSessionResourceToReleaseList,
		container,
	)
	if err != nil {
		return fmt.Errorf("error building handover command: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureHandoverCommand)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendInitialContextSetupRequest(
	ctx context.Context,
	amfUeNgapID int64,
	ranUeNgapID int64,
	ambrUplink string,
	ambrDownlink string,
	allowedNssai []models.Snssai,
	kgnb []byte,
	plmnID models.PlmnID,
	ueRadioCapability string,
	ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging,
	ueSecurityCapability *nasType.UESecurityCapability,
	nasPdu []byte,
	pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq,
	supportedGUAMI *models.Guami,
) error {
	pkt, err := buildInitialContextSetupRequest(
		amfUeNgapID,
		ranUeNgapID,
		ambrUplink,
		ambrDownlink,
		allowedNssai,
		kgnb,
		plmnID,
		ueRadioCapability,
		ueRadioCapabilityForPaging,
		ueSecurityCapability,
		nasPdu,
		pduSessionResourceSetupRequestList,
		supportedGUAMI,
	)
	if err != nil {
		return fmt.Errorf("error building initial context setup request: %s", err)
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureInitialContextSetupRequest)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendPathSwitchRequestAcknowledge(
	ctx context.Context,
	amfUeNgapID int64,
	ranUeNgapID int64,
	ueSecurityCapability *nasType.UESecurityCapability,
	ncc uint8,
	nh []byte,
	pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList,
	pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck,
	snssaiList []models.Snssai,
) error {
	pkt, err := buildPathSwitchRequestAcknowledge(
		amfUeNgapID,
		ranUeNgapID,
		ueSecurityCapability,
		ncc,
		nh,
		pduSessionResourceSwitchedList,
		pduSessionResourceReleasedList,
		snssaiList,
	)
	if err != nil {
		return fmt.Errorf("error building path switch request acknowledge: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedurePathSwitchRequestAcknowledge)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}

func (s *RealNGAPSender) SendHandoverRequest(
	ctx context.Context,
	amfUeNgapID int64,
	handOverType ngapType.HandoverType,
	uplinkAmbr string,
	downlinkAmbr string,
	ueSecurityCapability *nasType.UESecurityCapability,
	ncc uint8,
	nh []byte,
	cause ngapType.Cause,
	pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq,
	sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer,
	snssaiList []models.Snssai,
	supportedGUAMI *models.Guami,
) error {
	pkt, err := buildHandoverRequest(
		amfUeNgapID,
		handOverType,
		uplinkAmbr,
		downlinkAmbr,
		ueSecurityCapability,
		ncc,
		nh,
		cause,
		pduSessionResourceSetupListHOReq,
		sourceToTargetTransparentContainer,
		snssaiList,
		supportedGUAMI,
	)
	if err != nil {
		return fmt.Errorf("error building handover request: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedureHandoverRequest)
	if err != nil {
		return fmt.Errorf("send error: %s", err.Error())
	}

	return nil
}
