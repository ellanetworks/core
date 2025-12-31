package pdusession

import (
	"context"
	"fmt"

	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
)

func releaseTunnel(ctx context.Context, smf *smfContext.SMFContext, smContext *smfContext.SMContext) error {
	if smContext.Tunnel == nil {
		return fmt.Errorf("tunnel not found")
	}

	smContext.Tunnel.DataPath.DeactivateTunnelAndPDR()

	err := pfcp.SendPfcpSessionDeletionRequest(ctx, smf, smContext.Tunnel.DataPath.DPNode.UPF.NodeID, smContext)
	if err != nil {
		return fmt.Errorf("send PFCP session deletion request failed: %v", err)
	}

	smContext.Tunnel = nil

	return nil
}
