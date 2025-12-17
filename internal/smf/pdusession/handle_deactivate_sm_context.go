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

func DeactivateSmContext(ctx ctxt.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "SMF Deactivate SmContext")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	farList, err := handleUpCnxStateDeactivate(smContext)
	if err != nil {
		return fmt.Errorf("error handling UP connection state: %v", err)
	}

	sessionContext, exist := smContext.PFCPContext[smContext.Tunnel.DataPath.DPNode.UPF.NodeID.String()]
	if !exist {
		return fmt.Errorf("pfcp session context not found for upf: %s", smContext.Tunnel.DataPath.DPNode.UPF.NodeID.String())
	}

	err = pfcp.SendPfcpSessionModificationRequest(ctx, sessionContext.LocalSEID, sessionContext.RemoteSEID, nil, farList, nil)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return nil
}

func handleUpCnxStateDeactivate(smContext *context.SMContext) ([]*context.FAR, error) {
	if smContext.Tunnel == nil {
		return nil, nil
	}

	ANUPF := smContext.Tunnel.DataPath.DPNode
	if ANUPF.DownLinkTunnel.PDR == nil {
		return nil, fmt.Errorf("AN Release Error, PDR is nil")
	}

	ANUPF.DownLinkTunnel.PDR.FAR.State = context.RuleUpdate
	ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction.Forw = false
	ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction.Buff = true
	ANUPF.DownLinkTunnel.PDR.FAR.ApplyAction.Nocp = true

	if ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters != nil {
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = nil
	}

	farList := []*context.FAR{ANUPF.DownLinkTunnel.PDR.FAR}

	return farList, nil
}
