// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/ellanetworks/core/internal/smf/util"
)

// GTPTunnel represents the GTP tunnel information
type GTPTunnel struct {
	SrcEndPoint  *DataPathNode
	DestEndPoint *DataPathNode

	PDR  map[string]*PDR
	TEID uint32
}

type DataPathNode struct {
	UPF *UPF
	// DataPathToAN *DataPathDownLink
	// DataPathToDN map[string]*DataPathUpLink //uuid to DataPathLink

	UpLinkTunnel   *GTPTunnel
	DownLinkTunnel *GTPTunnel
	// for UE Routing Topology
	// for special case:
	// branching & leafnode

	// InUse                bool
	IsBranchingPoint bool
	// DLDataPathLinkForPSA *DataPathUpLink
	// BPUpLinkPDRs         map[string]*DataPathDownLink // uuid to UpLink
}

type DataPath struct {
	// Data Path Double Link List
	FirstDPNode *DataPathNode
	// meta data
	Destination       Destination
	Activated         bool
	IsDefaultPath     bool
	HasBranchingPoint bool
}

type DataPathPool map[int64]*DataPath

type Destination struct {
	DestinationIP   string
	DestinationPort string
	URL             string
}

func NewDataPathNode() *DataPathNode {
	node := &DataPathNode{
		UpLinkTunnel:   &GTPTunnel{PDR: make(map[string]*PDR)},
		DownLinkTunnel: &GTPTunnel{PDR: make(map[string]*PDR)},
	}
	return node
}

func (node *DataPathNode) AddNext(next *DataPathNode) {
	node.DownLinkTunnel.SrcEndPoint = next
}

func (node *DataPathNode) AddPrev(prev *DataPathNode) {
	node.UpLinkTunnel.SrcEndPoint = prev
}

func (node *DataPathNode) Next() *DataPathNode {
	if node.DownLinkTunnel == nil {
		return nil
	}
	next := node.DownLinkTunnel.SrcEndPoint
	return next
}

func (node *DataPathNode) Prev() *DataPathNode {
	if node.UpLinkTunnel == nil {
		return nil
	}
	prev := node.UpLinkTunnel.SrcEndPoint
	return prev
}

func (node *DataPathNode) ActivateUpLinkTunnel(smContext *SMContext) error {
	var err error
	var pdr *PDR
	var flowQer *QER
	node.UpLinkTunnel.SrcEndPoint = node.Prev()
	node.UpLinkTunnel.DestEndPoint = node

	destUPF := node.UPF

	// Iterate through PCC Rules to install PDRs
	pccRuleUpdate := smContext.SmPolicyUpdates[0].PccRuleUpdate

	if pccRuleUpdate != nil {
		addRules := pccRuleUpdate.GetAddPccRuleUpdate()

		for name, rule := range addRules {
			if pdr, err = destUPF.BuildCreatePdrFromPccRule(rule); err == nil {
				if flowQer, err = node.CreatePccRuleQer(smContext, rule.RefQosData[0]); err == nil {
					pdr.QER = append(pdr.QER, flowQer)
				}
				// Set PDR in Tunnel
				node.UpLinkTunnel.PDR[name] = pdr
			}
		}
	} else {
		// Default PDR
		if pdr, err = destUPF.AddPDR(); err != nil {
			return fmt.Errorf("add PDR failed: %s", err)
		} else {
			node.UpLinkTunnel.PDR["default"] = pdr
		}
	}

	if err = smContext.PutPDRtoPFCPSession(destUPF.NodeID, node.UpLinkTunnel.PDR); err != nil {
		logger.SmfLog.Errorln("put PDR Error:", err)
		return err
	}

	return nil
}

func (node *DataPathNode) ActivateDownLinkTunnel(smContext *SMContext) error {
	var err error
	var pdr *PDR
	var flowQer *QER
	node.DownLinkTunnel.SrcEndPoint = node.Next()
	node.DownLinkTunnel.DestEndPoint = node

	destUPF := node.UPF
	// Iterate through PCC Rules to install PDRs
	pccRuleUpdate := smContext.SmPolicyUpdates[0].PccRuleUpdate
	if pccRuleUpdate != nil {
		addRules := pccRuleUpdate.GetAddPccRuleUpdate()
		for name, rule := range addRules {
			if pdr, err = destUPF.BuildCreatePdrFromPccRule(rule); err == nil {
				// Add PCC Rule Qos Data QER
				if flowQer, err = node.CreatePccRuleQer(smContext, rule.RefQosData[0]); err == nil {
					pdr.QER = append(pdr.QER, flowQer)
				}
				// Set PDR in Tunnel
				node.DownLinkTunnel.PDR[name] = pdr
			}
		}
	} else {
		// Default PDR
		if pdr, err = destUPF.AddPDR(); err != nil {
			logger.SmfLog.Errorln("in ActivateDownLinkTunnel UPF IP:", node.UPF.NodeID.ResolveNodeIDToIP().String())
			logger.SmfLog.Errorln("allocate PDR Error:", err)
			return fmt.Errorf("add PDR failed: %s", err)
		} else {
			node.DownLinkTunnel.PDR["default"] = pdr
		}
	}

	// Put PDRs in PFCP session
	if err = smContext.PutPDRtoPFCPSession(destUPF.NodeID, node.DownLinkTunnel.PDR); err != nil {
		logger.SmfLog.Errorln("put PDR error:", err)
		return err
	}

	return nil
}

func (node *DataPathNode) DeactivateUpLinkTunnel(smContext *SMContext) {
	for name, pdr := range node.UpLinkTunnel.PDR {
		if pdr != nil {
			logger.SmfLog.Infof("deactivated UpLinkTunnel PDR name[%v], id[%v]", name, pdr.PDRID)

			// Remove PDR from PFCP Session
			smContext.RemovePDRfromPFCPSession(node.UPF.NodeID, pdr)

			// Remove of UPF
			node.UPF.RemovePDR(pdr)

			if far := pdr.FAR; far != nil {
				node.UPF.RemoveFAR(far)

				bar := far.BAR
				if bar != nil {
					node.UPF.RemoveBAR(bar)
				}
			}
			if qerList := pdr.QER; qerList != nil {
				for _, qer := range qerList {
					if qer != nil {
						node.UPF.RemoveQER(qer)
					}
				}
			}
		}
	}

	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) DeactivateDownLinkTunnel(smContext *SMContext) {
	for name, pdr := range node.DownLinkTunnel.PDR {
		if pdr != nil {
			logger.SmfLog.Infof("deactivated DownLinkTunnel PDR name[%v], id[%v]", name, pdr.PDRID)

			// Remove PDR from PFCP Session
			smContext.RemovePDRfromPFCPSession(node.UPF.NodeID, pdr)

			// Remove from UPF
			node.UPF.RemovePDR(pdr)

			if far := pdr.FAR; far != nil {
				node.UPF.RemoveFAR(far)

				bar := far.BAR
				if bar != nil {
					node.UPF.RemoveBAR(bar)
				}
			}
			if qerList := pdr.QER; qerList != nil {
				for _, qer := range qerList {
					if qer != nil {
						node.UPF.RemoveQER(qer)
					}
				}
			}
		}
	}

	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) GetNodeIP() (ip string) {
	ip = node.UPF.NodeID.ResolveNodeIDToIP().String()
	return
}

func (node *DataPathNode) IsANUPF() bool {
	if node.Prev() == nil {
		return true
	} else {
		return false
	}
}

func (node *DataPathNode) IsAnchorUPF() bool {
	if node.Next() == nil {
		return true
	} else {
		return false
	}
}

func (dataPathPool DataPathPool) GetDefaultPath() (dataPath *DataPath) {
	for _, path := range dataPathPool {
		if path.IsDefaultPath {
			dataPath = path
			return
		}
	}
	return
}

func (dataPath *DataPath) String() string {
	firstDPNode := dataPath.FirstDPNode

	var str string

	str += "DataPath Meta Information\n"
	str += "Activated: " + strconv.FormatBool(dataPath.Activated) + "\n"
	str += "IsDefault Path: " + strconv.FormatBool(dataPath.IsDefaultPath) + "\n"
	str += "Has Braching Point: " + strconv.FormatBool(dataPath.HasBranchingPoint) + "\n"
	str += "Destination IP: " + dataPath.Destination.DestinationIP + "\n"
	str += "Destination Port: " + dataPath.Destination.DestinationPort + "\n"

	str += "DataPath Routing Information\n"
	index := 1
	for curDPNode := firstDPNode; curDPNode != nil; curDPNode = curDPNode.Next() {
		str += strconv.Itoa(index) + "th Node in the Path\n"
		str += "Current UPF IP: " + curDPNode.GetNodeIP() + "\n"
		if curDPNode.Prev() != nil {
			str += "Previous UPF IP: " + curDPNode.Prev().GetNodeIP() + "\n"
		} else {
			str += "Previous UPF IP: None\n"
		}

		if curDPNode.Next() != nil {
			str += "Next UPF IP: " + curDPNode.Next().GetNodeIP() + "\n"
		} else {
			str += "Next UPF IP: None\n"
		}

		index++
	}

	return str
}

func (dataPath *DataPath) ActivateUlDlTunnel(smContext *SMContext) error {
	firstDPNode := dataPath.FirstDPNode
	for curDataPathNode := firstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		if err := curDataPathNode.ActivateUpLinkTunnel(smContext); err != nil {
			return fmt.Errorf("couldn't activate UpLinkTunnel: %s", err)
		}
		if err := curDataPathNode.ActivateDownLinkTunnel(smContext); err != nil {
			return fmt.Errorf("couldn't activate DownLinkTunnel: %s", err)
		}
	}
	return nil
}

func (node *DataPathNode) CreatePccRuleQer(smContext *SMContext, qosData string) (*QER, error) {
	smPolicyDec := smContext.SmPolicyUpdates[0].SmPolicyDecision
	refQos := qos.GetQoSDataFromPolicyDecision(smPolicyDec, qosData)

	// Get Flow Status
	gateStatus := GateOpen

	var flowQER *QER

	newQER, err := node.UPF.AddQER()
	if err != nil {
		return nil, fmt.Errorf("failed to add QER: %v", err)
	}
	newQER.QFI.QFI = qos.GetQosFlowIDFromQosID(refQos.QosID)

	// Flow Status
	newQER.GateStatus = &GateStatus{
		ULGate: gateStatus,
		DLGate: gateStatus,
	}

	// Rates
	newQER.MBR = &MBR{
		ULMBR: util.BitRateTokbps(refQos.MaxbrUl),
		DLMBR: util.BitRateTokbps(refQos.MaxbrDl),
	}

	flowQER = newQER

	return flowQER, nil
}

func (node *DataPathNode) CreateSessRuleQer(smContext *SMContext) (*QER, error) {
	var flowQER *QER

	sessionRule := smContext.SelectedSessionRule()

	// Get Default Qos-Data for the session
	smPolicyDec := smContext.SmPolicyUpdates[0].SmPolicyDecision

	defQosData := qos.GetDefaultQoSDataFromPolicyDecision(smPolicyDec)
	if defQosData == nil {
		return nil, fmt.Errorf("default QOS Data not found in Policy Decision")
	}
	if newQER, err := node.UPF.AddQER(); err != nil {
		logger.SmfLog.Errorln("new QER failed")
		return nil, err
	} else {
		newQER.QFI.QFI = qos.GetQosFlowIDFromQosID(defQosData.QosID)
		newQER.GateStatus = &GateStatus{
			ULGate: GateOpen,
			DLGate: GateOpen,
		}
		newQER.MBR = &MBR{
			ULMBR: util.BitRateTokbps(sessionRule.AuthSessAmbr.Uplink),
			DLMBR: util.BitRateTokbps(sessionRule.AuthSessAmbr.Downlink),
		}

		flowQER = newQER
	}

	return flowQER, nil
}

func (node *DataPathNode) ActivateUpLinkPdr(smContext *SMContext, defQER *QER, defPrecedence uint32) error {
	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = smContext.PDUAddress.IP.To4()

	curULTunnel := node.UpLinkTunnel
	for _, ULPDR := range curULTunnel.PDR {
		ULPDR.QER = append(ULPDR.QER, defQER)

		// Set Default precedence
		if ULPDR.Precedence == 0 {
			ULPDR.Precedence = defPrecedence
		}

		ULPDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceAccess}
		ULPDR.PDI.LocalFTeID = &FTEID{
			Ch: true,
		}
		ULPDR.PDI.UEIPAddress = &ueIPAddr
		ULPDR.PDI.NetworkInstance = smContext.Dnn

		ULPDR.OuterHeaderRemoval = &OuterHeaderRemoval{
			OuterHeaderRemovalDescription: OuterHeaderRemovalGtpUUdpIpv4,
		}

		ULFAR := ULPDR.FAR
		ULFAR.ApplyAction = ApplyAction{
			Buff: false,
			Drop: false,
			Dupl: false,
			Forw: true,
			Nocp: false,
		}
		ULFAR.ForwardingParameters = &ForwardingParameters{
			DestinationInterface: DestinationInterface{
				InterfaceValue: DestinationInterfaceCore,
			},
			NetworkInstance: smContext.Dnn,
		}

		if node.IsAnchorUPF() {
			ULFAR.ForwardingParameters.
				DestinationInterface.InterfaceValue = DestinationInterfaceSgiLanN6Lan
		}

		if nextULDest := node.Next(); nextULDest != nil {
			nextULTunnel := nextULDest.UpLinkTunnel
			iface := nextULTunnel.DestEndPoint.UPF.GetInterface(models.UpInterfaceTypeN9, smContext.Dnn)

			if upIP, err := iface.IP(smContext.SelectedPDUSessionType); err != nil {
				return fmt.Errorf("could not get IP address for Uplink PDR: %s", err)
			} else {
				ULFAR.ForwardingParameters.OuterHeaderCreation = &OuterHeaderCreation{
					OuterHeaderCreationDescription: OuterHeaderCreationGtpUUdpIpv4,
					IPv4Address:                    upIP,
					TeID:                           nextULTunnel.TEID,
				}
			}
		}
	}
	return nil
}

func (node *DataPathNode) ActivateDlLinkPdr(smContext *SMContext, defQER *QER, defPrecedence uint32, dataPath *DataPath) error {
	var iface *UPFInterfaceInfo
	curDLTunnel := node.DownLinkTunnel

	// UPF provided UE ip-addr
	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = smContext.PDUAddress.IP.To4()

	for _, DLPDR := range curDLTunnel.PDR {
		DLPDR.QER = append(DLPDR.QER, defQER)

		if DLPDR.Precedence == 0 {
			DLPDR.Precedence = defPrecedence
		}

		if !node.IsAnchorUPF() {
			DLPDR.OuterHeaderRemoval = &OuterHeaderRemoval{
				OuterHeaderRemovalDescription: OuterHeaderRemovalGtpUUdpIpv4,
			}
		}

		DLPDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
		DLPDR.PDI.UEIPAddress = &ueIPAddr
		DLFAR := DLPDR.FAR
		if nextDLDest := node.Prev(); nextDLDest != nil {
			nextDLTunnel := nextDLDest.DownLinkTunnel
			DLFAR.ApplyAction = ApplyAction{
				Buff: true,
				Drop: false,
				Dupl: false,
				Forw: false,
				Nocp: true,
			}

			iface = nextDLDest.UPF.GetInterface(models.UpInterfaceTypeN9, smContext.Dnn)

			if upIP, err := iface.IP(smContext.SelectedPDUSessionType); err != nil {
				return fmt.Errorf("could not get IP address for Downlink PDR: %s", err)
			} else {
				DLFAR.ForwardingParameters = &ForwardingParameters{
					DestinationInterface: DestinationInterface{InterfaceValue: DestinationInterfaceAccess},
					OuterHeaderCreation: &OuterHeaderCreation{
						OuterHeaderCreationDescription: OuterHeaderCreationGtpUUdpIpv4,
						IPv4Address:                    upIP,
						TeID:                           nextDLTunnel.TEID,
					},
				}
			}
		} else {
			if anIP := smContext.Tunnel.ANInformation.IPAddress; anIP != nil {
				ANUPF := dataPath.FirstDPNode
				DefaultDLPDR := ANUPF.DownLinkTunnel.PDR["default"]
				DLFAR := DefaultDLPDR.FAR
				DLFAR.ForwardingParameters = new(ForwardingParameters)
				DLFAR.ForwardingParameters.DestinationInterface.InterfaceValue = DestinationInterfaceAccess
				DLFAR.ForwardingParameters.NetworkInstance = smContext.Dnn
				DLFAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)

				dlOuterHeaderCreation := DLFAR.ForwardingParameters.OuterHeaderCreation
				dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
				dlOuterHeaderCreation.TeID = smContext.Tunnel.ANInformation.TEID
				dlOuterHeaderCreation.IPv4Address = smContext.Tunnel.ANInformation.IPAddress.To4()
			}
		}
	}
	return nil
}

func (dataPath *DataPath) ActivateTunnelAndPDR(smContext *SMContext, precedence uint32) error {
	err := smContext.AllocateLocalSEIDForDataPath(dataPath)
	if err != nil {
		return fmt.Errorf("could not allocate local SEID for DataPath: %s", err)
	}

	err = dataPath.ActivateUlDlTunnel(smContext)
	if err != nil {
		return fmt.Errorf("could not activate UL/DL Tunnel: %s", err)
	}

	// Activate PDR
	for curDataPathNode := dataPath.FirstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		// Add flow QER
		defQER, err := curDataPathNode.CreateSessRuleQer(smContext)
		if err != nil {
			return err
		}

		// Setup UpLink PDR
		if curDataPathNode.UpLinkTunnel != nil {
			if err := curDataPathNode.ActivateUpLinkPdr(smContext, defQER, precedence); err != nil {
				return fmt.Errorf("couldn't activate uplink pdr: %v", err)
			}
		}

		// Setup DownLink PDR
		if curDataPathNode.DownLinkTunnel != nil {
			if err := curDataPathNode.ActivateDlLinkPdr(smContext, defQER, precedence, dataPath); err != nil {
				return fmt.Errorf("couldn't activate downlink pdr: %v", err)
			}
		}

		ueIPAddr := UEIPAddress{}
		ueIPAddr.V4 = true
		ueIPAddr.IPv4Address = smContext.PDUAddress.IP.To4()

		if curDataPathNode.DownLinkTunnel != nil {
			if curDataPathNode.DownLinkTunnel.SrcEndPoint == nil {
				for _, DNDLPDR := range curDataPathNode.DownLinkTunnel.PDR {
					DNDLPDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
					DNDLPDR.PDI.NetworkInstance = smContext.Dnn
					DNDLPDR.PDI.UEIPAddress = &ueIPAddr
				}
			}
		}
	}

	dataPath.Activated = true
	return nil
}

func (dataPath *DataPath) DeactivateTunnelAndPDR(smContext *SMContext) {
	firstDPNode := dataPath.FirstDPNode

	// Deactivate Tunnels
	for curDataPathNode := firstDPNode; curDataPathNode != nil; curDataPathNode = curDataPathNode.Next() {
		curDataPathNode.DeactivateUpLinkTunnel(smContext)
		curDataPathNode.DeactivateDownLinkTunnel(smContext)
	}

	dataPath.Activated = false
}
