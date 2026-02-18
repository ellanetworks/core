package pdusession

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/nas"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*models.UpdateSmContextResponse, error) {
	ctx, span := tracer.Start(
		ctx,
		"SMF Update SmContext N1 Msg",
		trace.WithAttributes(
			attribute.String("smf.smContextRef", smContextRef),
		),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	rsp, sendPfcpDelete, err := handleUpdateN1Msg(ctx, n1Msg, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N1 message: %v", err)
	}

	if sendPfcpDelete {
		err := releaseTunnel(ctx, smf, smContext)
		if err != nil {
			return nil, fmt.Errorf("failed to release tunnel: %v", err)
		}
	}

	return rsp, nil
}

func handleUpdateN1Msg(ctx context.Context, n1Msg []byte, smContext *smfContext.SMContext) (*models.UpdateSmContextResponse, bool, error) {
	if n1Msg == nil {
		return nil, false, nil
	}

	m := nas.NewMessage()

	err := m.GsmMessageDecode(&n1Msg)
	if err != nil {
		return nil, false, fmt.Errorf("error decoding N1SmMessage: %v", err)
	}

	logger.SmfLog.Debug("Update SM Context Request N1SmMessage", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	switch m.GsmHeader.GetMessageType() {
	case nas.MsgTypePDUSessionReleaseRequest:
		logger.SmfLog.Info("N1 Msg PDU Session Release Request received", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

		smf := smfContext.SMFSelf()

		err := smf.ReleaseUeIPAddr(ctx, smContext.Supi)
		if err != nil {
			return nil, false, fmt.Errorf("failed to release UE IP Addr: %v", err)
		}

		pti := m.PDUSessionReleaseRequest.GetPTI()

		n1SmMsg, err := smfNas.BuildGSMPDUSessionReleaseCommand(smContext.PDUSessionID, pti)
		if err != nil {
			return nil, false, fmt.Errorf("build GSM PDUSessionReleaseCommand failed: %v", err)
		}

		n2SmMsg, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
		if err != nil {
			return nil, false, fmt.Errorf("build PDUSession Resource Release Command Transfer Error: %v", err)
		}

		sendPfcpDelete := false
		if smContext.Tunnel != nil {
			sendPfcpDelete = true
		}

		response := &models.UpdateSmContextResponse{
			BinaryDataN1SmMessage:     n1SmMsg,
			N2SmInfoTypePduResRel:     true,
			BinaryDataN2SmInformation: n2SmMsg,
		}

		return response, sendPfcpDelete, nil

	default:
		logger.SmfLog.Warn("N1 Msg type not supported in SM Context Update", zap.Uint8("MessageType", m.GsmHeader.GetMessageType()), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		return nil, false, nil
	}
}
