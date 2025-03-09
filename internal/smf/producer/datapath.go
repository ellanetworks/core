// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
)

type PFCPState struct {
	nodeID  context.NodeID
	pdrList []*context.PDR
	farList []*context.FAR
	qerList []*context.QER
}

// SendPFCPRules send all datapaths to UPFs
func SendPFCPRules(smContext *context.SMContext) context.PFCPSessionResponseStatus {
	pfcpPool := make(map[string]*PFCPState)

	var responseStatus context.PFCPSessionResponseStatus
	for _, dataPath := range smContext.Tunnel.DataPathPool {
		if dataPath.Activated {
			for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
				pdrList := make([]*context.PDR, 0, 2)
				farList := make([]*context.FAR, 0, 2)
				qerList := make([]*context.QER, 0, 2)

				if curDataPathNode.UpLinkTunnel != nil && curDataPathNode.UpLinkTunnel.PDR != nil {
					for _, pdr := range curDataPathNode.UpLinkTunnel.PDR {
						pdrList = append(pdrList, pdr)
						farList = append(farList, pdr.FAR)
						if pdr.QER != nil {
							qerList = append(qerList, pdr.QER...)
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
					}
				}

				pfcpState := pfcpPool[curDataPathNode.GetNodeIP()]
				if pfcpState == nil {
					pfcpPool[curDataPathNode.GetNodeIP()] = &PFCPState{
						nodeID:  curDataPathNode.UPF.NodeID,
						pdrList: pdrList,
						farList: farList,
						qerList: qerList,
					}
				} else {
					pfcpState.pdrList = append(pfcpState.pdrList, pdrList...)
					pfcpState.farList = append(pfcpState.farList, farList...)
					pfcpState.qerList = append(pfcpState.qerList, qerList...)
				}
			}
		}
	}
	for ip, pfcpState := range pfcpPool {
		sessionContext, exist := smContext.PFCPContext[ip]
		if !exist || sessionContext.RemoteSEID == 0 {
			addPduSessionAnchor, status, err := pfcp.SendPfcpSessionEstablishmentRequest(pfcpState.nodeID, smContext, pfcpState.pdrList, pfcpState.farList, nil, pfcpState.qerList)
			responseStatus = *status
			if err != nil {
				logger.SmfLog.Errorf("send pfcp session establishment request failed: %v for UPF[%v, %v]: ", err, pfcpState.nodeID, pfcpState.nodeID.ResolveNodeIDToIP())
			}
			logger.SmfLog.Infof("Sent PFCP session establishment request to upf: %v", pfcpState.nodeID)
			if addPduSessionAnchor {
				rspNodeID := context.NewNodeID("0.0.0.0")
				responseStatus = AddPDUSessionAnchorAndULCL(smContext, *rspNodeID)
			}
		} else {
			addPduSessionAnchor, status, err := pfcp.SendPfcpSessionModificationRequest(pfcpState.nodeID, smContext, pfcpState.pdrList, pfcpState.farList, nil, pfcpState.qerList)
			responseStatus = *status
			if err != nil {
				logger.SmfLog.Errorf("send pfcp session modification request failed: %v for UPF[%v, %v]: ", err, pfcpState.nodeID, pfcpState.nodeID.ResolveNodeIDToIP())
			}
			logger.SmfLog.Infof("Sent PFCP session modification request to upf: %v", pfcpState.nodeID)
			if addPduSessionAnchor {
				rspNodeID := context.NewNodeID("0.0.0.0")
				responseStatus = AddPDUSessionAnchorAndULCL(smContext, *rspNodeID)
			}
		}
	}
	return responseStatus
}
