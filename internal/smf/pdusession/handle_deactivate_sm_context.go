package pdusession

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func DeactivateSmContext(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "SMF Deactivate SmContext")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	farList, err := handleUpCnxStateDeactivate(smContext)
	if err != nil {
		return fmt.Errorf("error handling UP connection state: %v", err)
	}

	if smContext.PFCPContext == nil {
		return fmt.Errorf("pfcp session context not found for upf")
	}

	err = pfcp.SendPfcpSessionModificationRequest(ctx, smf, smContext.PFCPContext.LocalSEID, smContext.PFCPContext.RemoteSEID, nil, farList, nil)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return nil
}

func handleUpCnxStateDeactivate(smContext *smfContext.SMContext) ([]*smfContext.FAR, error) {
	if smContext.Tunnel == nil {
		return nil, nil
	}

	if smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR == nil {
		return nil, fmt.Errorf("AN Release Error, PDR is nil")
	}

	smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR.FAR.State = smfContext.RuleUpdate
	smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR.FAR.ApplyAction.Forw = false
	smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR.FAR.ApplyAction.Buff = true
	smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR.FAR.ApplyAction.Nocp = true

	if smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR.FAR.ForwardingParameters != nil {
		smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = nil
	}

	farList := []*smfContext.FAR{smContext.Tunnel.DataPath.DPNode.DownLinkTunnel.PDR.FAR}

	return farList, nil
}
