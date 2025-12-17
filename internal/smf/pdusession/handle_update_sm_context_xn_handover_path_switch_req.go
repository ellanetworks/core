package pdusession

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func UpdateSmContextXnHandoverPathSwitchReq(ctx ctxt.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "SMF Update SmContext Handover Path Switch Request")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pdrList, farList, n2buf, err := handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	sessionContext, exist := smContext.PFCPContext[smContext.Tunnel.DataPath.DPNode.UPF.NodeID.String()]
	if !exist {
		return nil, fmt.Errorf("pfcp session context not found for upf: %s", smContext.Tunnel.DataPath.DPNode.UPF.NodeID.String())
	}

	err = pfcp.SendPfcpSessionModificationRequest(ctx, sessionContext.LocalSEID, sessionContext.RemoteSEID, pdrList, farList, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return n2buf, nil
}

func handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data []byte, smContext *context.SMContext) ([]*context.PDR, []*context.FAR, []byte, error) {
	logger.SmfLog.Debug("handle Path Switch Request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	if err := context.HandlePathSwitchRequestTransfer(n2Data, smContext); err != nil {
		return nil, nil, nil, fmt.Errorf("handle PathSwitchRequestTransfer failed: %v", err)
	}

	n2Buf, err := context.BuildPathSwitchRequestAcknowledgeTransfer(smContext.Tunnel.DataPath.DPNode)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build Path Switch Transfer Error: %v", err)
	}

	pdrList := []*context.PDR{}
	farList := []*context.FAR{}
	dataPath := smContext.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		pdrList = append(pdrList, ANUPF.DownLinkTunnel.PDR)
		farList = append(farList, ANUPF.DownLinkTunnel.PDR.FAR)
	}

	return pdrList, farList, n2Buf, nil
}
