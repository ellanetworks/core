// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"go.uber.org/zap"
)

// SendPFCPRules send all datapaths to UPFs
func SendPFCPRules(ctx ctxt.Context, smContext *context.SMContext) error {
	dataPath := smContext.Tunnel.DataPath
	if !dataPath.Activated {
		logger.SmfLog.Debug("DataPath is not activated, skip sending PFCP rules")
		return nil
	}

	curDataPathNode := dataPath.DPNode

	pdrList := make([]*context.PDR, 0, 2)
	farList := make([]*context.FAR, 0, 2)
	qerList := make([]*context.QER, 0, 2)
	urrList := make([]*context.URR, 0, 2)

	if curDataPathNode.UpLinkTunnel != nil && curDataPathNode.UpLinkTunnel.PDR != nil {
		pdrList = append(pdrList, curDataPathNode.UpLinkTunnel.PDR)
		farList = append(farList, curDataPathNode.UpLinkTunnel.PDR.FAR)
		if curDataPathNode.UpLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, curDataPathNode.UpLinkTunnel.PDR.QER)
		}
		if curDataPathNode.UpLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, curDataPathNode.UpLinkTunnel.PDR.URR)
		}
	}

	if curDataPathNode.DownLinkTunnel != nil && curDataPathNode.DownLinkTunnel.PDR != nil {
		pdrList = append(pdrList, curDataPathNode.DownLinkTunnel.PDR)
		farList = append(farList, curDataPathNode.DownLinkTunnel.PDR.FAR)

		if curDataPathNode.DownLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, curDataPathNode.DownLinkTunnel.PDR.QER)
		}
		if curDataPathNode.DownLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, curDataPathNode.DownLinkTunnel.PDR.URR)
		}
	}

	nodeID := curDataPathNode.UPF.NodeID

	sessionContext, exist := smContext.PFCPContext[curDataPathNode.GetNodeIP()]
	if !exist || sessionContext.RemoteSEID == 0 {
		err := pfcp.SendPfcpSessionEstablishmentRequest(ctx, sessionContext.LocalSEID, pdrList, farList, nil, qerList, urrList)
		if err != nil {
			return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
		}

		logger.SmfLog.Info("Sent PFCP session establishment request to upf", zap.String("nodeID", nodeID.String()))

		return nil
	}

	err := pfcp.SendPfcpSessionModificationRequest(ctx, sessionContext.LocalSEID, sessionContext.RemoteSEID, pdrList, farList, nil, qerList)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request to upf", zap.String("nodeID", nodeID.String()))

	return nil
}
