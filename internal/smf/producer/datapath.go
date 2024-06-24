package producer

import (
	"github.com/omec-project/pfcp/pfcpType"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
	pfcp_message "github.com/yeastengine/ella/internal/smf/pfcp/message"
)

type PFCPState struct {
	nodeID  pfcpType.NodeID
	pdrList []*smf_context.PDR
	farList []*smf_context.FAR
	qerList []*smf_context.QER
	port    uint16
}

// SendPFCPRule send one datapath to UPF
func SendPFCPRule(smContext *smf_context.SMContext, dataPath *smf_context.DataPath) {
	logger.PduSessLog.Infoln("Send PFCP Rule")
	logger.PduSessLog.Infoln("DataPath: ", dataPath)
	for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		pdrList := make([]*smf_context.PDR, 0, 2)
		farList := make([]*smf_context.FAR, 0, 2)
		qerList := make([]*smf_context.QER, 0, 2)

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

		sessionContext, exist := smContext.PFCPContext[curDataPathNode.GetNodeIP()]
		if !exist || sessionContext.RemoteSEID == 0 {
			pfcp_message.SendPfcpSessionEstablishmentRequest(
				curDataPathNode.UPF.NodeID, smContext, pdrList, farList, nil, qerList, curDataPathNode.UPF.Port)
		} else {
			pfcp_message.SendPfcpSessionModificationRequest(
				curDataPathNode.UPF.NodeID, smContext, pdrList, farList, nil, qerList, curDataPathNode.UPF.Port)
		}
	}
}

// SendPFCPRules send all datapaths to UPFs
func SendPFCPRules(smContext *smf_context.SMContext) {
	pfcpPool := make(map[string]*PFCPState)

	for _, dataPath := range smContext.Tunnel.DataPathPool {
		if dataPath.Activated {
			for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
				pdrList := make([]*smf_context.PDR, 0, 2)
				farList := make([]*smf_context.FAR, 0, 2)
				qerList := make([]*smf_context.QER, 0, 2)

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
						port:    curDataPathNode.UPF.Port,
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
	for ip, pfcp := range pfcpPool {
		sessionContext, exist := smContext.PFCPContext[ip]
		if !exist || sessionContext.RemoteSEID == 0 {
			pfcp_message.SendPfcpSessionEstablishmentRequest(
				pfcp.nodeID, smContext, pfcp.pdrList, pfcp.farList, nil, pfcp.qerList, pfcp.port)
		} else {
			pfcp_message.SendPfcpSessionModificationRequest(
				pfcp.nodeID, smContext, pfcp.pdrList, pfcp.farList, nil, pfcp.qerList, pfcp.port)
		}
	}
}

func removeDataPath(datapath *smf_context.DataPath) {
	for curDPNode := datapath.FirstDPNode; curDPNode != nil; curDPNode = curDPNode.Next() {
		if curDPNode.DownLinkTunnel != nil && curDPNode.DownLinkTunnel.PDR != nil {
			for _, pdr := range curDPNode.DownLinkTunnel.PDR {
				pdr.State = smf_context.RULE_REMOVE
				pdr.FAR.State = smf_context.RULE_REMOVE
			}
		}
		if curDPNode.UpLinkTunnel != nil && curDPNode.UpLinkTunnel.PDR != nil {
			for _, pdr := range curDPNode.UpLinkTunnel.PDR {
				pdr.State = smf_context.RULE_REMOVE
				pdr.FAR.State = smf_context.RULE_REMOVE
			}
		}
	}
}

// UpdateDataPathToUPF update the datapath of the UPF
func UpdateDataPathToUPF(smContext *smf_context.SMContext, oldDataPath, updateDataPath *smf_context.DataPath) {
	if oldDataPath == nil {
		SendPFCPRule(smContext, updateDataPath)
		return
	} else {
		removeDataPath(oldDataPath)
		SendPFCPRule(smContext, updateDataPath)
	}
}
