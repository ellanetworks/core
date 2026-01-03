package pdusession

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	ctx, span := tracer.Start(
		ctx,
		"SMF Update SmContext Handover Path Switch Request",
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

	pdrList, farList, n2buf, err := handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		return nil, fmt.Errorf("pfcp session context not found for upf")
	}

	err = pfcp.SendPfcpSessionModificationRequest(ctx, smf, smContext.PFCPContext.LocalSEID, smContext.PFCPContext.RemoteSEID, pdrList, farList, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return n2buf, nil
}

func handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data []byte, smContext *smfContext.SMContext) ([]*smfContext.PDR, []*smfContext.FAR, []byte, error) {
	logger.SmfLog.Debug("handle Path Switch Request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	if err := smfContext.HandlePathSwitchRequestTransfer(n2Data, smContext); err != nil {
		return nil, nil, nil, fmt.Errorf("handle PathSwitchRequestTransfer failed: %v", err)
	}

	n2Buf, err := smfContext.BuildPathSwitchRequestAcknowledgeTransfer(smContext.Tunnel.DataPath.DPNode)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build Path Switch Transfer Error: %v", err)
	}

	pdrList := []*smfContext.PDR{}
	farList := []*smfContext.FAR{}

	dataPath := smContext.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		pdrList = append(pdrList, ANUPF.DownLinkTunnel.PDR)
		farList = append(farList, ANUPF.DownLinkTunnel.PDR.FAR)
	}

	return pdrList, farList, n2Buf, nil
}
