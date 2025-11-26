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

type PFCPState struct {
	nodeID  context.NodeID
	pdrList []*context.PDR
	farList []*context.FAR
	qerList []*context.QER
	urrList []*context.URR
}

// SendPFCPRules send all datapaths to UPFs
func SendPFCPRules(ctx ctxt.Context, smContext *context.SMContext) error {
	pfcpPool := make(map[string]*PFCPState)
	dataPath := smContext.Tunnel.DataPath
	if dataPath.Activated {
		curDataPathNode := dataPath.DPNode
		pdrList := make([]*context.PDR, 0, 2)
		farList := make([]*context.FAR, 0, 2)
		qerList := make([]*context.QER, 0, 2)
		urrList := make([]*context.URR, 0, 2)

		if curDataPathNode.UpLinkTunnel != nil && curDataPathNode.UpLinkTunnel.PDR != nil {
			for _, pdr := range curDataPathNode.UpLinkTunnel.PDR {
				pdrList = append(pdrList, pdr)
				farList = append(farList, pdr.FAR)
				if pdr.QER != nil {
					qerList = append(qerList, pdr.QER...)
				}
				if pdr.URR != nil {
					urrList = append(urrList, pdr.URR)
				}
			}
		}
		if curDataPathNode.DownLinkTunnel != nil && curDataPathNode.DownLinkTunnel.PDR != nil {
			for _, pdr := range curDataPathNode.DownLinkTunnel.PDR {
				pdrList = append(pdrList, pdr)
				farList = append(farList, pdr.FAR)

				if pdr.QER != nil {
					qerList = append(qerList, pdr.QER...)
				}
				if pdr.URR != nil {
					urrList = append(urrList, pdr.URR)
				}
			}
		}

		pfcpState := pfcpPool[curDataPathNode.GetNodeIP()]
		if pfcpState == nil {
			pfcpPool[curDataPathNode.GetNodeIP()] = &PFCPState{
				nodeID:  curDataPathNode.UPF.NodeID,
				pdrList: pdrList,
				farList: farList,
				qerList: qerList,
				urrList: urrList,
			}
		} else {
			pfcpState.pdrList = append(pfcpState.pdrList, pdrList...)
			pfcpState.farList = append(pfcpState.farList, farList...)
			pfcpState.qerList = append(pfcpState.qerList, qerList...)
			pfcpState.urrList = append(pfcpState.urrList, urrList...)
		}
	}

	for ip, pfcpState := range pfcpPool {
		sessionContext, exist := smContext.PFCPContext[ip]
		if !exist || sessionContext.RemoteSEID == 0 {
			err := pfcp.SendPfcpSessionEstablishmentRequest(ctx, pfcpState.nodeID, smContext, pfcpState.pdrList, pfcpState.farList, nil, pfcpState.qerList, pfcpState.urrList)
			if err != nil {
				return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
			}
			logger.SmfLog.Info("Sent PFCP session establishment request to upf", zap.String("nodeID", pfcpState.nodeID.String()))
		} else {
			err := pfcp.SendPfcpSessionModificationRequest(ctx, pfcpState.nodeID, smContext, pfcpState.pdrList, pfcpState.farList, nil, pfcpState.qerList)
			if err != nil {
				logger.SmfLog.Error("send pfcp session modification request failed", zap.Error(err), zap.String("nodeID", pfcpState.nodeID.String()))
				return fmt.Errorf("failed to send PFCP session modification request: %v", err)
			}
			logger.SmfLog.Info("Sent PFCP session modification request to upf", zap.String("nodeID", pfcpState.nodeID.String()))
		}
	}
	return nil
}
