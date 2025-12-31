// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
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

func (node *DataPathNode) ActivateUpLinkTunnel() error {
	pdr, err := node.UPF.AddPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	node.UpLinkTunnel.PDR = pdr

	return nil
}

func (node *DataPathNode) ActivateDownLinkTunnel() error {
	pdr, err := node.UPF.AddPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	node.DownLinkTunnel.PDR = pdr

	return nil
}

func (node *DataPathNode) DeactivateUpLinkTunnel() {
	if node.UpLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in UpLink Tunnel")
		return
	}

	node.UPF.RemovePDR(node.UpLinkTunnel.PDR)

	if far := node.UpLinkTunnel.PDR.FAR; far != nil {
		node.UPF.RemoveFAR(far)
	}

	qer := node.UpLinkTunnel.PDR.QER
	if qer != nil {
		node.UPF.RemoveQER(qer)
	}

	urr := node.UpLinkTunnel.PDR.URR
	if urr != nil {
		node.UPF.RemoveURR(urr)
	}

	logger.SmfLog.Info("deactivated UpLinkTunnel PDR ")

	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) DeactivateDownLinkTunnel() {
	if node.DownLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in Downlink Tunnel")
		return
	}

	logger.SmfLog.Info("deactivated DownLinkTunnel PDR", zap.Uint16("pdrId", node.DownLinkTunnel.PDR.PDRID))

	node.UPF.RemovePDR(node.DownLinkTunnel.PDR)

	if far := node.DownLinkTunnel.PDR.FAR; far != nil {
		node.UPF.RemoveFAR(far)
	}

	qer := node.DownLinkTunnel.PDR.QER
	if qer != nil {
		node.UPF.RemoveQER(qer)
	}

	urr := node.DownLinkTunnel.PDR.URR
	if urr != nil {
		node.UPF.RemoveURR(urr)
	}

	node.DownLinkTunnel = &GTPTunnel{}
}

func (dataPath *DataPath) ActivateUlDlTunnel() error {
	err := dataPath.DPNode.ActivateUpLinkTunnel()
	if err != nil {
		return fmt.Errorf("couldn't activate UpLinkTunnel: %s", err)
	}

	err = dataPath.DPNode.ActivateDownLinkTunnel()
	if err != nil {
		return fmt.Errorf("couldn't activate DownLinkTunnel: %s", err)
	}

	return nil
}

func (node *DataPathNode) CreateSessRuleQer(smContext *SMContext) (*QER, error) {
	smPolicyDec := smContext.SmPolicyUpdates.SmPolicyDecision

	if smPolicyDec.QosDecs == nil {
		return nil, fmt.Errorf("QOS Data not found in Policy Decision")
	}

	newQER, err := node.UPF.AddQER()
	if err != nil {
		return nil, fmt.Errorf("failed to add QER: %v", err)
	}

	sessionRule := SelectedSessionRule(smContext.SmPolicyUpdates, smContext.SmPolicyData)

	newQER.QFI = smPolicyDec.QosDecs.QFI
	newQER.GateStatus = &GateStatus{
		ULGate: GateOpen,
		DLGate: GateOpen,
	}
	newQER.MBR = &MBR{
		ULMBR: BitRateTokbps(sessionRule.AuthSessAmbr.Uplink),
		DLMBR: BitRateTokbps(sessionRule.AuthSessAmbr.Downlink),
	}

	flowQER := newQER

	return flowQER, nil
}

func (node *DataPathNode) CreateSessRuleURR() (*URR, error) {
	return node.UPF.AddURR()
}

func (node *DataPathNode) ActivateUpLinkPdr(dnn string, pduAddress net.IP, defQER *QER, defURR *URR, defPrecedence uint32) {
	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = pduAddress.To4()

	curULTunnel := node.UpLinkTunnel

	curULTunnel.PDR.QER = defQER
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
	curULTunnel.PDR.PDI.NetworkInstance = dnn

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
		NetworkInstance: dnn,
	}

	ULFAR.ForwardingParameters.DestinationInterface.InterfaceValue = DestinationInterfaceSgiLanN6Lan
}

func (node *DataPathNode) ActivateDlLinkPdr(smContext *SMContext, pduAddress net.IP, defQER *QER, defURR *URR, defPrecedence uint32, dataPath *DataPath) {
	curDLTunnel := node.DownLinkTunnel

	// UPF provided UE ip-addr
	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = pduAddress.To4()

	curDLTunnel.PDR.QER = defQER
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
}

func (dataPath *DataPath) ActivateTunnelAndPDR(smf *SMFContext, smContext *SMContext, pduAddress net.IP, precedence uint32) error {
	smf.AllocateLocalSEIDForDataPath(smContext)

	err := dataPath.ActivateUlDlTunnel()
	if err != nil {
		return fmt.Errorf("could not activate UL/DL Tunnel: %s", err)
	}

	defQER, err := dataPath.DPNode.CreateSessRuleQer(smContext)
	if err != nil {
		return fmt.Errorf("failed to create default QER: %v", err)
	}

	defULURR, err := dataPath.DPNode.CreateSessRuleURR()
	if err != nil {
		return fmt.Errorf("failed to create uplink URR: %v", err)
	}

	defDLURR, err := dataPath.DPNode.CreateSessRuleURR()
	if err != nil {
		return fmt.Errorf("failed to create uplink URR: %v", err)
	}

	// Setup UpLink PDR
	if dataPath.DPNode.UpLinkTunnel != nil {
		dataPath.DPNode.ActivateUpLinkPdr(smContext.Dnn, pduAddress, defQER, defULURR, precedence)
	}

	// Setup DownLink PDR
	if dataPath.DPNode.DownLinkTunnel != nil {
		dataPath.DPNode.ActivateDlLinkPdr(smContext, pduAddress, defQER, defDLURR, precedence, dataPath)
	}

	ueIPAddr := UEIPAddress{}
	ueIPAddr.V4 = true
	ueIPAddr.IPv4Address = pduAddress.To4()

	if dataPath.DPNode.DownLinkTunnel != nil {
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.NetworkInstance = smContext.Dnn
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.UEIPAddress = &ueIPAddr
	}

	dataPath.Activated = true
	return nil
}

func (dataPath *DataPath) DeactivateTunnelAndPDR() {
	DPNode := dataPath.DPNode
	DPNode.DeactivateUpLinkTunnel()
	DPNode.DeactivateDownLinkTunnel()
	dataPath.Activated = false
}

func BitRateTokbps(bitrate string) uint64 {
	s := strings.Split(bitrate, " ")
	var kbps uint64

	var digit int

	if n, err := strconv.Atoi(s[0]); err != nil {
		return 0
	} else {
		digit = n
	}

	switch s[1] {
	case "bps":
		kbps = uint64(digit / 1000)
	case "Kbps":
		kbps = uint64(digit * 1)
	case "Mbps":
		kbps = uint64(digit * 1000)
	case "Gbps":
		kbps = uint64(digit * 1000000)
	case "Tbps":
		kbps = uint64(digit * 1000000000)
	}
	return kbps
}
