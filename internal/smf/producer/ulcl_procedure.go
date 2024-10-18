package producer

import (
	"net"
	"reflect"

	"github.com/omec-project/util/flowdesc"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/pfcp/message"
)

func AddPDUSessionAnchorAndULCL(smContext *context.SMContext, nodeID context.NodeID) {
	bpMGR := smContext.BPManager
	pendingUPF := bpMGR.PendingUPF

	switch bpMGR.AddingPSAState {
	case context.ActivatingDataPath:
		// select PSA2
		bpMGR.SelectPSA2(smContext)
		smContext.AllocateLocalSEIDForDataPath(bpMGR.ActivatingPath)
		// select an upf as ULCL
		err := bpMGR.FindULCL(smContext)
		if err != nil {
			logger.PduSessLog.Errorln(err)
			return
		}

		// Allocate Path PDR and TEID
		err = bpMGR.ActivatingPath.ActivateTunnelAndPDR(smContext, 255)
		if err != nil {
			logger.PduSessLog.Errorln(err)
		}
		// N1N2MessageTransfer Here

		// Establish PSA2
		EstablishPSA2(smContext)
	case context.EstablishingNewPSA:

		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.CtxLog.Warnln("In AddPDUSessionAnchorAndULCL case EstablishingNewPSA")
			logger.CtxLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return
		}

		if pendingUPF.IsEmpty() {
			EstablishRANTunnelInfo(smContext)
			// Establish ULCL
			EstablishULCL(smContext)
		}

	case context.EstablishingULCL:

		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.CtxLog.Warnln("In AddPDUSessionAnchorAndULCL case EstablishingULCL")
			logger.CtxLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return
		}

		if pendingUPF.IsEmpty() {
			UpdatePSA2DownLink(smContext)
		}

	case context.UpdatingPSA2DownLink:

		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.CtxLog.Warnln("In AddPDUSessionAnchorAndULCL case EstablishingULCL")
			logger.CtxLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return
		}

		if pendingUPF.IsEmpty() {
			UpdateRANAndIUPFUpLink(smContext)
		}
	case context.UpdatingRANAndIUPFUpLink:
		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.CtxLog.Warnln("In AddPDUSessionAnchorAndULCL case UpdatingRANAndIUPFUpLink")
			logger.CtxLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return
		}

		if pendingUPF.IsEmpty() {
			bpMGR.AddingPSAState = context.Finished
			bpMGR.BPStatus = context.AddPSASuccess
			logger.CtxLog.Infoln("[SMF] Add PSA success")
		}
	}
}

func EstablishPSA2(smContext *context.SMContext) {
	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	activatingPath := bpMGR.ActivatingPath
	ulcl := bpMGR.ULCL
	logger.PduSessLog.Infoln("In EstablishPSA2")
	nodeAfterULCL := false
	for curDataPathNode := activatingPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		if nodeAfterULCL {
			addr := net.UDPAddr{
				IP:   curDataPathNode.UPF.NodeID.NodeIdValue,
				Port: factory.UPF_PFCP_PORT,
			}

			logger.PduSessLog.Traceln("Send to upf addr: ", addr.String())

			upLinkPDR := curDataPathNode.UpLinkTunnel.PDR["default"] // TODO: Iterate over all PDRs

			pdrList := []*context.PDR{upLinkPDR}
			farList := []*context.FAR{upLinkPDR.FAR}
			barList := []*context.BAR{}
			qerList := upLinkPDR.QER

			lastNode := curDataPathNode.Prev()

			if lastNode != nil && !reflect.DeepEqual(lastNode.UPF.NodeID, ulcl.NodeID) {
				downLinkPDR := curDataPathNode.DownLinkTunnel.PDR["default"] // TODO: Iterate over all PDRs
				pdrList = append(pdrList, downLinkPDR)
				farList = append(farList, downLinkPDR.FAR)
			}

			curDPNodeIP := curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String()
			bpMGR.PendingUPF[curDPNodeIP] = true
			err := message.SendPfcpSessionEstablishmentRequest(
				curDataPathNode.UPF.NodeID, smContext, pdrList, farList, barList, qerList, curDataPathNode.UPF.Port)
			if err != nil {
				logger.PduSessLog.Errorf("send pfcp session establishment request failed: %v for UPF[%v, %v]: ", err, curDataPathNode.UPF.NodeID, curDataPathNode.UPF.NodeID.ResolveNodeIdToIp())
			}
		} else {
			if reflect.DeepEqual(curDataPathNode.UPF.NodeID, ulcl.NodeID) {
				nodeAfterULCL = true
			}
		}
	}

	bpMGR.AddingPSAState = context.EstablishingNewPSA
	logger.PduSessLog.Traceln("End of EstablishPSA2")
}

func EstablishULCL(smContext *context.SMContext) {
	logger.PduSessLog.Infoln("In EstablishULCL")

	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	activatingPath := bpMGR.ActivatingPath
	dest := activatingPath.Destination
	ulcl := bpMGR.ULCL

	// find updatedUPF in activatingPath
	for curDPNode := activatingPath.FirstDPNode; curDPNode != nil; curDPNode = curDPNode.Next() {
		if reflect.DeepEqual(ulcl.NodeID, curDPNode.UPF.NodeID) {
			UPLinkPDR := curDPNode.UpLinkTunnel.PDR["default"]     // TODO: Iterate over all PDRs
			DownLinkPDR := curDPNode.DownLinkTunnel.PDR["default"] // TODO: Iterate over all PDRs
			UPLinkPDR.State = context.RULE_INITIAL

			FlowDespcription := flowdesc.NewIPFilterRule()
			err := FlowDespcription.SetAction(flowdesc.Permit) // permit
			if err != nil {
				logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetDirection(flowdesc.Out) // uplink
			if err != nil {
				logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetDestinationIP(dest.DestinationIP)
			if err != nil {
				logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetDestinationPorts(dest.DestinationPort)
			if err != nil {
				logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetSourceIP(smContext.PDUAddress.Ip.To4().String())
			if err != nil {
				logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}

			FlowDespcriptionStr, err := flowdesc.Encode(FlowDespcription)
			if err != nil {
				logger.PduSessLog.Errorf("Error occurs when encoding flow despcription: %s\n", err)
			}

			UPLinkPDR.PDI.SDFFilter = &context.SDFFilter{
				Bid:                     false,
				Fl:                      false,
				Spi:                     false,
				Ttc:                     false,
				Fd:                      true,
				LengthOfFlowDescription: uint16(len(FlowDespcriptionStr)),
				FlowDescription:         []byte(FlowDespcriptionStr),
			}

			UPLinkPDR.Precedence = 30

			pdrList := []*context.PDR{UPLinkPDR, DownLinkPDR}
			farList := []*context.FAR{UPLinkPDR.FAR, DownLinkPDR.FAR}
			barList := []*context.BAR{}
			qerList := UPLinkPDR.QER

			curDPNodeIP := ulcl.NodeID.ResolveNodeIdToIp().String()
			bpMGR.PendingUPF[curDPNodeIP] = true
			err = message.SendPfcpSessionModificationRequest(ulcl.NodeID, smContext, pdrList, farList, barList, qerList, ulcl.Port)
			if err != nil {
				logger.PduSessLog.Errorf("send pfcp session modification request failed: %v for UPF[%v, %v]: ", err, ulcl.NodeID, ulcl.NodeID.ResolveNodeIdToIp())
			}
			break
		}
	}

	bpMGR.AddingPSAState = context.EstablishingULCL
	logger.PfcpLog.Info("[SMF] Establish ULCL msg has been send")
}

func UpdatePSA2DownLink(smContext *context.SMContext) {
	logger.PduSessLog.Traceln("In UpdatePSA2DownLink")

	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	ulcl := bpMGR.ULCL
	activatingPath := bpMGR.ActivatingPath

	farList := []*context.FAR{}
	pdrList := []*context.PDR{}
	barList := []*context.BAR{}
	qerList := []*context.QER{}

	for curDataPathNode := activatingPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		lastNode := curDataPathNode.Prev()

		if lastNode != nil {
			if reflect.DeepEqual(lastNode.UPF.NodeID, ulcl.NodeID) {
				downLinkPDR := curDataPathNode.DownLinkTunnel.PDR["default"] // TODO: Iterate over all PDRs
				downLinkPDR.State = context.RULE_INITIAL
				downLinkPDR.FAR.State = context.RULE_INITIAL

				pdrList = append(pdrList, downLinkPDR)
				farList = append(farList, downLinkPDR.FAR)
				qerList = append(qerList, downLinkPDR.QER...)

				curDPNodeIP := curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String()
				bpMGR.PendingUPF[curDPNodeIP] = true
				err := message.SendPfcpSessionModificationRequest(
					curDataPathNode.UPF.NodeID, smContext, pdrList, farList, barList, qerList, curDataPathNode.UPF.Port)
				if err != nil {
					logger.PduSessLog.Errorf("send pfcp session modification request failed: %v for UPF[%v, %v]: ", err, curDataPathNode.UPF.NodeID, curDataPathNode.UPF.NodeID.ResolveNodeIdToIp())
				}
				logger.PfcpLog.Info("[SMF] Update PSA2 downlink msg has been send")
				break
			}
		}
	}

	bpMGR.AddingPSAState = context.UpdatingPSA2DownLink
}

func EstablishRANTunnelInfo(smContext *context.SMContext) {
	logger.PduSessLog.Traceln("In UpdatePSA2DownLink")

	bpMGR := smContext.BPManager
	activatingPath := bpMGR.ActivatingPath

	defaultPath := smContext.Tunnel.DataPathPool.GetDefaultPath()
	defaultANUPF := defaultPath.FirstDPNode

	activatingANUPF := activatingPath.FirstDPNode

	// Uplink ANUPF In TEID
	activatingANUPF.UpLinkTunnel.TEID = defaultANUPF.UpLinkTunnel.TEID
	activatingANUPF.UpLinkTunnel.PDR["default"].PDI.LocalFTeid.Teid = defaultANUPF.UpLinkTunnel.PDR["default"].PDI.LocalFTeid.Teid // TODO: Iterate over all PDRs
	logger.AppLog.Warnf("activatingANUPF.UpLinkTunnel.PDR[\"default\"].PDI.LocalFTeid.Teid: %d\n", activatingANUPF.UpLinkTunnel.PDR["default"].PDI.LocalFTeid.Teid)

	// Downlink ANUPF OutTEID

	defaultANUPFDLFAR := defaultANUPF.DownLinkTunnel.PDR["default"].FAR       // TODO: Iterate over all PDRs
	activatingANUPFDLFAR := activatingANUPF.DownLinkTunnel.PDR["default"].FAR // TODO: Iterate over all PDRs
	activatingANUPFDLFAR.ApplyAction = context.ApplyAction{
		Buff: false,
		Drop: false,
		Dupl: false,
		Forw: true,
		Nocp: false,
	}
	activatingANUPFDLFAR.ForwardingParameters = &context.ForwardingParameters{
		DestinationInterface: context.DestinationInterface{
			InterfaceValue: context.DestinationInterfaceAccess,
		},
		NetworkInstance: []byte(smContext.Dnn),
	}

	activatingANUPFDLFAR.State = context.RULE_INITIAL
	activatingANUPFDLFAR.ForwardingParameters.OuterHeaderCreation = new(context.OuterHeaderCreation)
	anOuterHeaderCreation := activatingANUPFDLFAR.ForwardingParameters.OuterHeaderCreation
	anOuterHeaderCreation.OuterHeaderCreationDescription = context.OuterHeaderCreationGtpUUdpIpv4
	anOuterHeaderCreation.Teid = defaultANUPFDLFAR.ForwardingParameters.OuterHeaderCreation.Teid
	anOuterHeaderCreation.Ipv4Address = defaultANUPFDLFAR.ForwardingParameters.OuterHeaderCreation.Ipv4Address
}

func UpdateRANAndIUPFUpLink(smContext *context.SMContext) {
	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	activatingPath := bpMGR.ActivatingPath
	dest := activatingPath.Destination
	ulcl := bpMGR.ULCL

	for curDPNode := activatingPath.FirstDPNode; curDPNode != nil; curDPNode = curDPNode.Next() {
		if reflect.DeepEqual(ulcl.NodeID, curDPNode.UPF.NodeID) {
			break
		} else {
			UPLinkPDR := curDPNode.UpLinkTunnel.PDR["default"]     // TODO: Iterate over all PDRs
			DownLinkPDR := curDPNode.DownLinkTunnel.PDR["default"] // TODO: Iterate over all PDRs
			UPLinkPDR.State = context.RULE_INITIAL
			DownLinkPDR.State = context.RULE_INITIAL

			if _, exist := bpMGR.UpdatedBranchingPoint[curDPNode.UPF]; exist {
				// add SDF Filter
				FlowDespcription := flowdesc.NewIPFilterRule()
				err := FlowDespcription.SetAction(flowdesc.Permit) // permit
				if err != nil {
					logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetDirection(flowdesc.Out) // uplink
				if err != nil {
					logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetDestinationIP(dest.DestinationIP)
				if err != nil {
					logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetDestinationPorts(dest.DestinationPort)
				if err != nil {
					logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetSourceIP(smContext.PDUAddress.Ip.To4().String())
				if err != nil {
					logger.PduSessLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}

				FlowDespcriptionStr, err := flowdesc.Encode(FlowDespcription)
				if err != nil {
					logger.PduSessLog.Errorf("Error occurs when encoding flow despcription: %s\n", err)
				}

				UPLinkPDR.PDI.SDFFilter = &context.SDFFilter{
					Bid:                     false,
					Fl:                      false,
					Spi:                     false,
					Ttc:                     false,
					Fd:                      true,
					LengthOfFlowDescription: uint16(len(FlowDespcriptionStr)),
					FlowDescription:         []byte(FlowDespcriptionStr),
				}
			}

			pdrList := []*context.PDR{UPLinkPDR, DownLinkPDR}
			farList := []*context.FAR{UPLinkPDR.FAR, DownLinkPDR.FAR}
			barList := []*context.BAR{}
			qerList := UPLinkPDR.QER

			curDPNodeIP := curDPNode.UPF.NodeID.ResolveNodeIdToIp().String()
			bpMGR.PendingUPF[curDPNodeIP] = true
			err := message.SendPfcpSessionModificationRequest(curDPNode.UPF.NodeID, smContext, pdrList, farList, barList, qerList, curDPNode.UPF.Port)
			if err != nil {
				logger.PduSessLog.Errorf("send pfcp session modification request failed: %v for UPF[%v, %v]: ", err, curDPNode.UPF.NodeID, curDPNode.UPF.NodeID.ResolveNodeIdToIp())
			}
		}
	}

	if bpMGR.PendingUPF.IsEmpty() {
		bpMGR.AddingPSAState = context.Finished
		bpMGR.BPStatus = context.AddPSASuccess
		logger.CtxLog.Infoln("[SMF] Add PSA success")
	} else {
		bpMGR.AddingPSAState = context.UpdatingRANAndIUPFUpLink
	}
}
