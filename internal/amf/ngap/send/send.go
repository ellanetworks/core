package send

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/ngap/send")

type RealNGAPSender struct {
	Conn *sctp.SCTPConn
}

func (s *RealNGAPSender) SendToRan(ctx context.Context, packet []byte, msgType NGAPProcedure) error {
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

	if len(packet) == 0 {
		return fmt.Errorf("packet len is 0")
	}

	if s.Conn == nil {
		return fmt.Errorf("ran conn is nil")
	}

	if s.Conn.RemoteAddr() == nil {
		return fmt.Errorf("ran address is nil")
	}

	info := sctp.SndRcvInfo{
		Stream: sid,
		PPID:   nativeToNetworkEndianness32(sctp.NGAPPPID),
	}
	if _, err := s.Conn.SCTPWrite(packet, &info); err != nil {
		return fmt.Errorf("send write to sctp connection: %s", err.Error())
	}

	logger.LogNetworkEvent(
		ctx,
		logger.NGAPNetworkProtocol,
		string(msgType),
		logger.DirectionOutbound,
		s.Conn.LocalAddr().String(),
		s.Conn.RemoteAddr().String(),
		packet,
	)

	return nil
}

type PlmnSupportItem struct {
	PlmnID models.PlmnID
	SNssai *models.Snssai
}

func (s *RealNGAPSender) SendNGSetupResponse(ctx context.Context, guami *models.Guami, plmnSupported *models.PlmnSupportItem, amfName string, amfRelativeCapacity int64) error {
	pkt, err := buildNGSetupResponse(guami, plmnSupported, amfName, amfRelativeCapacity)
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

// criticality ->from received node when received node can't comprehend the IE or missing IE
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

// criticality ->from received node when received node can't comprehend the IE or missing IE
// If the AMF cannot accept the update,
// it shall respond with a RAN CONFIGURATION UPDATE FAILURE message and appropriate cause value.
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

// SONConfigurationTransfer = sONConfigurationTransfer from uplink Ran Configuration Transfer
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

// pduSessionResourceReleasedList: provided by AMF, and the transfer data is from SMF
// criticalityDiagnostics: from received node when received not comprehended IE or missing IE
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

// An AMF shall be able to instruct other peer CP NFs, subscribed to receive such a notification,
// that it will be unavailable on this AMF and its corresponding target AMF(s).
// If CP NF does not subscribe to receive AMF unavailable notification, the CP NF may attempt
// forwarding the transaction towards the old AMF and detect that the AMF is unavailable. When
// it detects unavailable, it marks the AMF and its associated GUAMI(s) as unavailable.
// Defined in 23.501 5.21.2.2.2
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

// cause = initiate the Handover Cancel procedure with the appropriate value for the Cause IE.
// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
// when received node can't comprehend the IE or missing IE
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

// pduSessionResourceHandoverList: provided by amf and transfer is return from smf
// pduSessionResourceToReleaseList: provided by amf and transfer is return from smf
// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
// when received node can't comprehend the IE or missing IE
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
	allowedNssai *models.Snssai,
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

func nativeToNetworkEndianness32(value uint32) uint32 {
	var b [4]byte
	binary.NativeEndian.PutUint32(b[:], value)
	return binary.BigEndian.Uint32(b[:])
}
