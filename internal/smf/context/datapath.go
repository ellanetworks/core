// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/ellanetworks/core/internal/smf/util"
	"go.uber.org/zap"
)

// GTPTunnel represents the GTP tunnel information
type GTPTunnel struct {
	PDR  *PDR
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
	pdr, err := node.UPF.AddPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	node.UpLinkTunnel.PDR = pdr

	return nil
}

func (node *DataPathNode) ActivateDownLinkTunnel(smContext *SMContext) error {
	pdr, err := node.UPF.AddPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	node.DownLinkTunnel.PDR = pdr

	return nil
}

func (node *DataPathNode) DeactivateUpLinkTunnel(smContext *SMContext) {
	if node.UpLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in UpLink Tunnel")
		return
	}

	// Remove of UPF
	node.UPF.RemovePDR(node.UpLinkTunnel.PDR)

	if far := node.UpLinkTunnel.PDR.FAR; far != nil {
		node.UPF.RemoveFAR(far)

		bar := far.BAR
		if bar != nil {
			node.UPF.RemoveBAR(bar)
		}
	}

	if qerList := node.UpLinkTunnel.PDR.QER; qerList != nil {
		for _, qer := range qerList {
			if qer != nil {
				node.UPF.RemoveQER(qer)
			}
		}
	}

	logger.SmfLog.Info("deactivated UpLinkTunnel PDR ")

	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) DeactivateDownLinkTunnel(smContext *SMContext) {
	if node.DownLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in Downlink Tunnel")
		return
	}

	logger.SmfLog.Info("deactivated DownLinkTunnel PDR", zap.Uint16("pdrId", node.DownLinkTunnel.PDR.PDRID))

	// Remove from UPF
	node.UPF.RemovePDR(node.DownLinkTunnel.PDR)

	if far := node.DownLinkTunnel.PDR.FAR; far != nil {
		node.UPF.RemoveFAR(far)

		bar := far.BAR
		if bar != nil {
			node.UPF.RemoveBAR(bar)
		}
	}

	if qerList := node.DownLinkTunnel.PDR.QER; qerList != nil {
		for _, qer := range qerList {
			if qer != nil {
				node.UPF.RemoveQER(qer)
			}
		}
	}

	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) GetNodeIP() string {
	return node.UPF.NodeID.String()
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
		newQER.QFI = defQosData.QFI
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

func (node *DataPathNode) ActivateUpLinkPdr(smContext *SMContext, defQER *QER, defURR *URR, defPrecedence uint32) error {
	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = smContext.PDUAddress.To4()

	curULTunnel := node.UpLinkTunnel

	curULTunnel.PDR.QER = append(curULTunnel.PDR.QER, defQER)
	curULTunnel.PDR.URR = defURR

	// Set Default precedence
	if curULTunnel.PDR.Precedence == 0 {
		curULTunnel.PDR.Precedence = defPrecedence
	}

	curULTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceAccess}
	curULTunnel.PDR.PDI.LocalFTeID = &FTEID{
		Ch: true,
	}
	curULTunnel.PDR.PDI.UEIPAddress = &ueIPAddr
	curULTunnel.PDR.PDI.NetworkInstance = smContext.Dnn

	curULTunnel.PDR.OuterHeaderRemoval = &OuterHeaderRemoval{
		OuterHeaderRemovalDescription: OuterHeaderRemovalGtpUUdpIpv4,
	}

	ULFAR := curULTunnel.PDR.FAR
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
	return nil
}

func (node *DataPathNode) ActivateDlLinkPdr(smContext *SMContext, defQER *QER, defURR *URR, defPrecedence uint32, dataPath *DataPath) error {
	curDLTunnel := node.DownLinkTunnel

	// UPF provided UE ip-addr
	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = smContext.PDUAddress.To4()

	curDLTunnel.PDR.QER = append(curDLTunnel.PDR.QER, defQER)
	curDLTunnel.PDR.URR = defURR

	if curDLTunnel.PDR.Precedence == 0 {
		curDLTunnel.PDR.Precedence = defPrecedence
	}

	curDLTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
	curDLTunnel.PDR.PDI.UEIPAddress = &ueIPAddr
	if anIP := smContext.Tunnel.ANInformation.IPAddress; anIP != nil {
		ANUPF := dataPath.DPNode
		DefaultDLPDR := ANUPF.DownLinkTunnel.PDR
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

	defULURR := &URR{
		URRID: 1,
		MeasurementMethods: MeasurementMethods{
			Volume: true,
		},
		ReportingTriggers: ReportingTriggers{
			PeriodicReporting: true,
		},
		MeasurementPeriod: 60 * time.Second,
	}

	defDLURR := &URR{
		URRID: 2,
		MeasurementMethods: MeasurementMethods{
			Volume: true,
		},
		ReportingTriggers: ReportingTriggers{
			PeriodicReporting: true,
		},
		MeasurementPeriod: 60 * time.Second,
	}

	// Setup UpLink PDR
	if dataPath.DPNode.UpLinkTunnel != nil {
		if err := dataPath.DPNode.ActivateUpLinkPdr(smContext, defQER, defULURR, precedence); err != nil {
			return fmt.Errorf("couldn't activate uplink pdr: %v", err)
		}
	}

	// Setup DownLink PDR
	if dataPath.DPNode.DownLinkTunnel != nil {
		if err := dataPath.DPNode.ActivateDlLinkPdr(smContext, defQER, defDLURR, precedence, dataPath); err != nil {
			return fmt.Errorf("couldn't activate downlink pdr: %v", err)
		}
	}

	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = smContext.PDUAddress.To4()

	if dataPath.DPNode.DownLinkTunnel != nil {
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.NetworkInstance = smContext.Dnn
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.UEIPAddress = &ueIPAddr
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
