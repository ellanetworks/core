package pdusession

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
)

func releaseTunnel(ctx ctxt.Context, smContext *context.SMContext) error {
	if smContext.Tunnel == nil {
		return fmt.Errorf("tunnel not found")
	}

	smContext.Tunnel.DataPath.DeactivateTunnelAndPDR()

	err := pfcp.SendPfcpSessionDeletionRequest(ctx, smContext.Tunnel.DataPath.DPNode.UPF.NodeID, smContext)
	if err != nil {
		return fmt.Errorf("send PFCP session deletion request failed: %v", err)
	}

	smContext.Tunnel = nil

	return nil
}
