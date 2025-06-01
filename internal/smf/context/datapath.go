// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/ellanetworks/core/internal/smf/util"
	"go.uber.org/zap"
)

// GTPTunnel represents the GTP tunnel information
type GTPTunnel struct {
	PDR  map[string]*PDR
	TEID uint32
}

type DataPathNode struct {
	UPF            *UPF
	UpLinkTunnel   *GTPTunnel
	DownLinkTunnel *GTPTunnel
}

type DataPath struct {
	DPNode    *DataPathNode
	Activated bool
}

func (node *DataPathNode) ActivateUpLinkTunnel(smContext *SMContext) error {
	var err error
	var pdr *PDR
	var flowQer *QER

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
		logger.SmfLog.Error("couldn't put PDR to PFCP session", zap.Error(err))
		return err
	}

	return nil
}

func (node *DataPathNode) ActivateDownLinkTunnel(smContext *SMContext) error {
	var err error
	var pdr *PDR
	var flowQer *QER

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
		pdr, err = destUPF.AddPDR()
		if err != nil {
			return fmt.Errorf("add PDR failed: %s", err)
		}
		node.DownLinkTunnel.PDR["default"] = pdr
	}

	// Put PDRs in PFCP session
	if err = smContext.PutPDRtoPFCPSession(destUPF.NodeID, node.DownLinkTunnel.PDR); err != nil {
		return fmt.Errorf("error in put PDR to PFCP session: %s", err)
	}

	return nil
}

func (node *DataPathNode) DeactivateUpLinkTunnel(smContext *SMContext) {
	for name, pdr := range node.UpLinkTunnel.PDR {
		if pdr == nil {
			logger.SmfLog.Debug("PDR is nil in UpLink Tunnel", zap.String("name", name))
			continue
		}

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
		logger.SmfLog.Info("deactivated UpLinkTunnel PDR ", zap.String("name", name), zap.Uint16("id", pdr.PDRID))
	}

	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) DeactivateDownLinkTunnel(smContext *SMContext) {
	for name, pdr := range node.DownLinkTunnel.PDR {
		if pdr != nil {
			logger.SmfLog.Info("deactivated DownLinkTunnel PDR", zap.String("name", name), zap.Uint16("id", pdr.PDRID))

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

func (dataPath *DataPath) ActivateUlDlTunnel(smContext *SMContext) error {
	DPNode := dataPath.DPNode
	err := DPNode.ActivateUpLinkTunnel(smContext)
	if err != nil {
		return fmt.Errorf("couldn't activate UpLinkTunnel: %s", err)
	}
	err = DPNode.ActivateDownLinkTunnel(smContext)
	if err != nil {
		return fmt.Errorf("couldn't activate DownLinkTunnel: %s", err)
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
	qfi, err := qos.GetQosFlowIDFromQosID(refQos.QosID)
	if err != nil {
		return nil, fmt.Errorf("failed to get QosFlowID from QosID %s: %v", refQos.QosID, err)
	}
	newQER.QFI.QFI = qfi

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
		logger.SmfLog.Error("new QER failed")
		return nil, err
	} else {
		qfi, err := qos.GetQosFlowIDFromQosID(defQosData.QosID)
		if err != nil {
			return nil, fmt.Errorf("failed to get QosFlowID from QosID %s: %v", defQosData.QosID, err)
		}

		newQER.QFI.QFI = qfi
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

		ULFAR.ForwardingParameters.DestinationInterface.InterfaceValue = DestinationInterfaceSgiLanN6Lan
	}
	return nil
}

func (node *DataPathNode) ActivateDlLinkPdr(smContext *SMContext, defQER *QER, defPrecedence uint32, dataPath *DataPath) error {
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

		DLPDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
		DLPDR.PDI.UEIPAddress = &ueIPAddr
		if anIP := smContext.Tunnel.ANInformation.IPAddress; anIP != nil {
			ANUPF := dataPath.DPNode
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
	// Add flow QER
	defQER, err := dataPath.DPNode.CreateSessRuleQer(smContext)
	if err != nil {
		return err
	}

	// Setup UpLink PDR
	if dataPath.DPNode.UpLinkTunnel != nil {
		if err := dataPath.DPNode.ActivateUpLinkPdr(smContext, defQER, precedence); err != nil {
			return fmt.Errorf("couldn't activate uplink pdr: %v", err)
		}
	}

	// Setup DownLink PDR
	if dataPath.DPNode.DownLinkTunnel != nil {
		if err := dataPath.DPNode.ActivateDlLinkPdr(smContext, defQER, precedence, dataPath); err != nil {
			return fmt.Errorf("couldn't activate downlink pdr: %v", err)
		}
	}

	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = smContext.PDUAddress.IP.To4()

	if dataPath.DPNode.DownLinkTunnel != nil {
		for _, DNDLPDR := range dataPath.DPNode.DownLinkTunnel.PDR {
			DNDLPDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
			DNDLPDR.PDI.NetworkInstance = smContext.Dnn
			DNDLPDR.PDI.UEIPAddress = &ueIPAddr
		}
	}

	dataPath.Activated = true
	return nil
}

func (dataPath *DataPath) DeactivateTunnelAndPDR(smContext *SMContext) {
	DPNode := dataPath.DPNode
	DPNode.DeactivateUpLinkTunnel(smContext)
	DPNode.DeactivateDownLinkTunnel(smContext)
	dataPath.Activated = false
}
