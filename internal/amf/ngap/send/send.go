package send

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
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
	pkt, err := BuildNGSetupResponse(ctx, guami, plmnSupported, amfName, amfRelativeCapacity)
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
	pkt, err := BuildNGSetupFailure(cause)
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
	if partOfNGInterface != nil && len(partOfNGInterface.List) == 0 {
		return fmt.Errorf("length of partOfNGInterface is 0")
	}

	pkt, err := BuildNGResetAcknowledge(partOfNGInterface)
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
	pkt, err := BuildErrorIndication(amfUeNgapID, ranUeNgapID, cause, criticalityDiagnostics)
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
	pkt, err := BuildRanConfigurationUpdateAcknowledge(criticalityDiagnostics)
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
	pkt, err := BuildRanConfigurationUpdateFailure(cause, criticalityDiagnostics)
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
	pkt, err := BuildDownlinkRanConfigurationTransfer(transfer)
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
	pkt, err := BuildPathSwitchRequestFailure(amfUeNgapID, ranUeNgapID, pduSessionResourceReleasedList, criticalityDiagnostics)
	if err != nil {
		return fmt.Errorf("error building path switch request failure: %s", err.Error())
	}

	err = s.SendToRan(ctx, pkt, NGAPProcedurePathSwitchRequestFailure)
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
