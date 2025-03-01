// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"reflect"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/smf/util"
)

func AddPDUSessionAnchorAndULCL(smContext *context.SMContext, nodeID context.NodeID) context.PFCPSessionResponseStatus {
	bpMGR := smContext.BPManager
	pendingUPF := bpMGR.PendingUPF
	var status context.PFCPSessionResponseStatus

	switch bpMGR.AddingPSAState {
	case context.ActivatingDataPath:
		// select PSA2
		bpMGR.SelectPSA2(smContext)
		err := smContext.AllocateLocalSEIDForDataPath(bpMGR.ActivatingPath)
		if err != nil {
			logger.SmfLog.Errorln(err)
			return status
		}
		// select an upf as ULCL
		err = bpMGR.FindULCL(smContext)
		if err != nil {
			logger.SmfLog.Errorln(err)
			return status
		}

		// Allocate Path PDR and TEID
		err = bpMGR.ActivatingPath.ActivateTunnelAndPDR(smContext, 255)
		if err != nil {
			logger.SmfLog.Errorln(err)
		}
		// N1N2MessageTransfer Here

		// Establish PSA2
		status = EstablishPSA2(smContext)
	case context.EstablishingNewPSA:

		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.SmfLog.Warnln("In AddPDUSessionAnchorAndULCL case EstablishingNewPSA")
			logger.SmfLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return status
		}

		if pendingUPF.IsEmpty() {
			EstablishRANTunnelInfo(smContext)
			// Establish ULCL
			status = EstablishULCL(smContext)
		}

	case context.EstablishingULCL:

		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.SmfLog.Warnln("In AddPDUSessionAnchorAndULCL case EstablishingULCL")
			logger.SmfLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return status
		}

		if pendingUPF.IsEmpty() {
			status = UpdatePSA2DownLink(smContext)
		}

	case context.UpdatingPSA2DownLink:

		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.SmfLog.Warnln("In AddPDUSessionAnchorAndULCL case EstablishingULCL")
			logger.SmfLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return status
		}

		if pendingUPF.IsEmpty() {
			status = UpdateRANAndIUPFUpLink(smContext)
		}
	case context.UpdatingRANAndIUPFUpLink:
		trggierUPFIP := nodeID.ResolveNodeIdToIp().String()
		_, exist := pendingUPF[trggierUPFIP]

		if exist {
			delete(pendingUPF, trggierUPFIP)
		} else {
			logger.SmfLog.Warnln("In AddPDUSessionAnchorAndULCL case UpdatingRANAndIUPFUpLink")
			logger.SmfLog.Warnln("UPF IP ", trggierUPFIP, " doesn't exist in pending UPF!")
			return status
		}

		if pendingUPF.IsEmpty() {
			bpMGR.AddingPSAState = context.Finished
			bpMGR.BPStatus = context.AddPSASuccess
			logger.SmfLog.Infoln("[SMF] Add PSA success")
		}
	}
	return status
}

func EstablishPSA2(smContext *context.SMContext) context.PFCPSessionResponseStatus {
	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	activatingPath := bpMGR.ActivatingPath
	ulcl := bpMGR.ULCL
	nodeAfterULCL := false
	var responseStatus context.PFCPSessionResponseStatus
	for curDataPathNode := activatingPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		if nodeAfterULCL {
			upLinkPDR := curDataPathNode.UpLinkTunnel.PDR["default"]

			pdrList := []*context.PDR{upLinkPDR}
			farList := []*context.FAR{upLinkPDR.FAR}
			barList := []*context.BAR{}
			qerList := upLinkPDR.QER

			lastNode := curDataPathNode.Prev()

			if lastNode != nil && !reflect.DeepEqual(lastNode.UPF.NodeID, ulcl.NodeID) {
				downLinkPDR := curDataPathNode.DownLinkTunnel.PDR["default"]
				pdrList = append(pdrList, downLinkPDR)
				farList = append(farList, downLinkPDR.FAR)
			}

			curDPNodeIP := curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String()
			bpMGR.PendingUPF[curDPNodeIP] = true
			addPduSessionAnchor, status, err := pfcp.SendPfcpSessionEstablishmentRequest(
				curDataPathNode.UPF.NodeID, smContext, pdrList, farList, barList, qerList)
			responseStatus = *status
			if err != nil {
				logger.SmfLog.Errorf("send pfcp session establishment request failed: %v for UPF[%v, %v]: ", err, curDataPathNode.UPF.NodeID, curDataPathNode.UPF.NodeID.ResolveNodeIdToIp())
			}
			if addPduSessionAnchor {
				rspNodeID := context.NewNodeID("0.0.0.0")
				responseStatus = AddPDUSessionAnchorAndULCL(smContext, *rspNodeID)
			}
		} else {
			if reflect.DeepEqual(curDataPathNode.UPF.NodeID, ulcl.NodeID) {
				nodeAfterULCL = true
			}
		}
	}

	bpMGR.AddingPSAState = context.EstablishingNewPSA
	logger.SmfLog.Debugln("End of EstablishPSA2")
	return responseStatus
}

func EstablishULCL(smContext *context.SMContext) context.PFCPSessionResponseStatus {
	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	activatingPath := bpMGR.ActivatingPath
	dest := activatingPath.Destination
	ulcl := bpMGR.ULCL
	var responseStatus context.PFCPSessionResponseStatus

	// find updatedUPF in activatingPath
	for curDPNode := activatingPath.FirstDPNode; curDPNode != nil; curDPNode = curDPNode.Next() {
		if reflect.DeepEqual(ulcl.NodeID, curDPNode.UPF.NodeID) {
			UPLinkPDR := curDPNode.UpLinkTunnel.PDR["default"]
			DownLinkPDR := curDPNode.DownLinkTunnel.PDR["default"]
			UPLinkPDR.State = context.RULE_INITIAL

			FlowDespcription := util.NewIPFilterRule()
			err := FlowDespcription.SetAction(util.Permit) // permit
			if err != nil {
				logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetDirection(util.Out) // uplink
			if err != nil {
				logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetDestinationIP(dest.DestinationIP)
			if err != nil {
				logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetDestinationPorts(dest.DestinationPort)
			if err != nil {
				logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}
			err = FlowDespcription.SetSourceIP(smContext.PDUAddress.Ip.To4().String())
			if err != nil {
				logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
			}

			FlowDespcriptionStr, err := util.Encode(FlowDespcription)
			if err != nil {
				logger.SmfLog.Errorf("Error occurs when encoding flow despcription: %s\n", err)
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
			addPduSessionAnchor, status, err := pfcp.SendPfcpSessionModificationRequest(ulcl.NodeID, smContext, pdrList, farList, barList, qerList)
			responseStatus = *status
			if err != nil {
				logger.SmfLog.Errorf("send pfcp session modification request failed: %v for UPF[%v, %v]: ", err, ulcl.NodeID, ulcl.NodeID.ResolveNodeIdToIp())
			}
			if addPduSessionAnchor {
				rspNodeID := context.NewNodeID("0.0.0.0")
				responseStatus = AddPDUSessionAnchorAndULCL(smContext, *rspNodeID)
			}
			break
		}
	}

	bpMGR.AddingPSAState = context.EstablishingULCL
	logger.SmfLog.Info("[SMF] Establish ULCL msg has been send")
	return responseStatus
}

func UpdatePSA2DownLink(smContext *context.SMContext) context.PFCPSessionResponseStatus {
	logger.SmfLog.Debugln("In UpdatePSA2DownLink")

	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	ulcl := bpMGR.ULCL
	activatingPath := bpMGR.ActivatingPath
	var responseStatus context.PFCPSessionResponseStatus

	farList := []*context.FAR{}
	pdrList := []*context.PDR{}
	barList := []*context.BAR{}
	qerList := []*context.QER{}

	for curDataPathNode := activatingPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		lastNode := curDataPathNode.Prev()

		if lastNode != nil {
			if reflect.DeepEqual(lastNode.UPF.NodeID, ulcl.NodeID) {
				downLinkPDR := curDataPathNode.DownLinkTunnel.PDR["default"]
				downLinkPDR.State = context.RULE_INITIAL
				downLinkPDR.FAR.State = context.RULE_INITIAL

				pdrList = append(pdrList, downLinkPDR)
				farList = append(farList, downLinkPDR.FAR)
				qerList = append(qerList, downLinkPDR.QER...)

				curDPNodeIP := curDataPathNode.UPF.NodeID.ResolveNodeIdToIp().String()
				bpMGR.PendingUPF[curDPNodeIP] = true
				addPduSessionAnchor, status, err := pfcp.SendPfcpSessionModificationRequest(
					curDataPathNode.UPF.NodeID, smContext, pdrList, farList, barList, qerList)
				responseStatus = *status
				if err != nil {
					logger.SmfLog.Errorf("send pfcp session modification request failed: %v for UPF[%v, %v]: ", err, curDataPathNode.UPF.NodeID, curDataPathNode.UPF.NodeID.ResolveNodeIdToIp())
				}
				if addPduSessionAnchor {
					rspNodeID := context.NewNodeID("0.0.0.0")
					responseStatus = AddPDUSessionAnchorAndULCL(smContext, *rspNodeID)
				}
				logger.SmfLog.Info("[SMF] Update PSA2 downlink msg has been send")
				break
			}
		}
	}

	bpMGR.AddingPSAState = context.UpdatingPSA2DownLink
	return responseStatus
}

func EstablishRANTunnelInfo(smContext *context.SMContext) {
	logger.SmfLog.Debugln("In UpdatePSA2DownLink")

	bpMGR := smContext.BPManager
	activatingPath := bpMGR.ActivatingPath

	defaultPath := smContext.Tunnel.DataPathPool.GetDefaultPath()
	defaultANUPF := defaultPath.FirstDPNode

	activatingANUPF := activatingPath.FirstDPNode

	// Uplink ANUPF In TEID
	activatingANUPF.UpLinkTunnel.TEID = defaultANUPF.UpLinkTunnel.TEID
	activatingANUPF.UpLinkTunnel.PDR["default"].PDI.LocalFTeid.Teid = defaultANUPF.UpLinkTunnel.PDR["default"].PDI.LocalFTeid.Teid
	logger.SmfLog.Warnf("activatingANUPF.UpLinkTunnel.PDR[\"default\"].PDI.LocalFTeid.Teid: %d\n", activatingANUPF.UpLinkTunnel.PDR["default"].PDI.LocalFTeid.Teid)

	// Downlink ANUPF OutTEID

	defaultANUPFDLFAR := defaultANUPF.DownLinkTunnel.PDR["default"].FAR
	activatingANUPFDLFAR := activatingANUPF.DownLinkTunnel.PDR["default"].FAR
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

func UpdateRANAndIUPFUpLink(smContext *context.SMContext) context.PFCPSessionResponseStatus {
	bpMGR := smContext.BPManager
	bpMGR.PendingUPF = make(context.PendingUPF)
	activatingPath := bpMGR.ActivatingPath
	dest := activatingPath.Destination
	ulcl := bpMGR.ULCL
	var responseStatus context.PFCPSessionResponseStatus

	for curDPNode := activatingPath.FirstDPNode; curDPNode != nil; curDPNode = curDPNode.Next() {
		if reflect.DeepEqual(ulcl.NodeID, curDPNode.UPF.NodeID) {
			break
		} else {
			UPLinkPDR := curDPNode.UpLinkTunnel.PDR["default"]
			DownLinkPDR := curDPNode.DownLinkTunnel.PDR["default"]
			UPLinkPDR.State = context.RULE_INITIAL
			DownLinkPDR.State = context.RULE_INITIAL

			if _, exist := bpMGR.UpdatedBranchingPoint[curDPNode.UPF]; exist {
				// add SDF Filter
				FlowDespcription := util.NewIPFilterRule()
				err := FlowDespcription.SetAction(util.Permit) // permit
				if err != nil {
					logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetDirection(util.Out) // uplink
				if err != nil {
					logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetDestinationIP(dest.DestinationIP)
				if err != nil {
					logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetDestinationPorts(dest.DestinationPort)
				if err != nil {
					logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}
				err = FlowDespcription.SetSourceIP(smContext.PDUAddress.Ip.To4().String())
				if err != nil {
					logger.SmfLog.Errorf("Error occurs when setting flow despcription: %s\n", err)
				}

				FlowDespcriptionStr, err := util.Encode(FlowDespcription)
				if err != nil {
					logger.SmfLog.Errorf("Error occurs when encoding flow despcription: %s\n", err)
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
			addPduSessionAnchor, status, err := pfcp.SendPfcpSessionModificationRequest(curDPNode.UPF.NodeID, smContext, pdrList, farList, barList, qerList)
			responseStatus = *status
			if err != nil {
				logger.SmfLog.Errorf("send pfcp session modification request failed: %v for UPF[%v, %v]: ", err, curDPNode.UPF.NodeID, curDPNode.UPF.NodeID.ResolveNodeIdToIp())
			}
			if addPduSessionAnchor {
				rspNodeID := context.NewNodeID("0.0.0.0")
				responseStatus = AddPDUSessionAnchorAndULCL(smContext, *rspNodeID)
			}
		}
	}

	if bpMGR.PendingUPF.IsEmpty() {
		bpMGR.AddingPSAState = context.Finished
		bpMGR.BPStatus = context.AddPSASuccess
		logger.SmfLog.Infoln("[SMF] Add PSA success")
	} else {
		bpMGR.AddingPSAState = context.UpdatingRANAndIUPFUpLink
	}
	return responseStatus
}
