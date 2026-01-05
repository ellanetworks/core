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
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// GTPTunnel represents the GTP tunnel information
type GTPTunnel struct {
	PDR  *PDR
	TEID uint32
	N3IP net.IP
}

type DataPathNode struct {
	UpLinkTunnel   *GTPTunnel
	DownLinkTunnel *GTPTunnel
}

type DataPath struct {
	DPNode    *DataPathNode
	Activated bool
}

func (node *DataPathNode) ActivateUpLinkTunnel(smf *SMF) error {
	pdr, err := smf.AddPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	node.UpLinkTunnel.PDR = pdr

	return nil
}

func (node *DataPathNode) ActivateDownLinkTunnel(smf *SMF) error {
	pdr, err := smf.AddPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	node.DownLinkTunnel.PDR = pdr

	return nil
}

func (node *DataPathNode) DeactivateUpLinkTunnel(smf *SMF) {
	if node.UpLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in UpLink Tunnel")
		return
	}

	smf.RemovePDR(node.UpLinkTunnel.PDR)

	if far := node.UpLinkTunnel.PDR.FAR; far != nil {
		smf.RemoveFAR(far)
	}

	qer := node.UpLinkTunnel.PDR.QER
	if qer != nil {
		smf.RemoveQER(qer)
	}

	urr := node.UpLinkTunnel.PDR.URR
	if urr != nil {
		smf.RemoveURR(urr)
	}

	logger.SmfLog.Info("deactivated UpLinkTunnel PDR ")

	node.DownLinkTunnel = &GTPTunnel{}
}

func (node *DataPathNode) DeactivateDownLinkTunnel(smf *SMF) {
	if node.DownLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in Downlink Tunnel")
		return
	}

	logger.SmfLog.Info("deactivated DownLinkTunnel PDR", zap.Uint16("pdrId", node.DownLinkTunnel.PDR.PDRID))

	smf.RemovePDR(node.DownLinkTunnel.PDR)

	if far := node.DownLinkTunnel.PDR.FAR; far != nil {
		smf.RemoveFAR(far)
	}

	qer := node.DownLinkTunnel.PDR.QER
	if qer != nil {
		smf.RemoveQER(qer)
	}

	urr := node.DownLinkTunnel.PDR.URR
	if urr != nil {
		smf.RemoveURR(urr)
	}

	node.DownLinkTunnel = &GTPTunnel{}
}

func (dataPath *DataPath) ActivateUlDlTunnel(smf *SMF) error {
	err := dataPath.DPNode.ActivateUpLinkTunnel(smf)
	if err != nil {
		return fmt.Errorf("couldn't activate UpLinkTunnel: %s", err)
	}

	err = dataPath.DPNode.ActivateDownLinkTunnel(smf)
	if err != nil {
		return fmt.Errorf("couldn't activate DownLinkTunnel: %s", err)
	}

	return nil
}

func (node *DataPathNode) CreateSessRuleQer(smf *SMF, smPolicyDecision *models.SmPolicyDecision) (*QER, error) {
	if smPolicyDecision.QosData == nil {
		return nil, fmt.Errorf("QOS Data not found in Policy Decision")
	}

	newQER, err := smf.AddQER()
	if err != nil {
		return nil, fmt.Errorf("failed to add QER: %v", err)
	}

	newQER.QFI = smPolicyDecision.QosData.QFI
	newQER.GateStatus = &GateStatus{
		ULGate: GateOpen,
		DLGate: GateOpen,
	}
	newQER.MBR = &MBR{
		ULMBR: BitRateTokbps(smPolicyDecision.SessionRule.AuthSessAmbr.Uplink),
		DLMBR: BitRateTokbps(smPolicyDecision.SessionRule.AuthSessAmbr.Downlink),
	}

	flowQER := newQER

	return flowQER, nil
}

func (node *DataPathNode) ActivateUpLinkPdr(dnn string, pduAddress net.IP, defQER *QER, defURR *URR, defPrecedence uint32) {
	node.UpLinkTunnel.PDR.QER = defQER
	node.UpLinkTunnel.PDR.URR = defURR

	// Set Default precedence
	if node.UpLinkTunnel.PDR.Precedence == 0 {
		node.UpLinkTunnel.PDR.Precedence = defPrecedence
	}

	node.UpLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceAccess}
	node.UpLinkTunnel.PDR.PDI.LocalFTeID = &FTEID{
		Ch: true,
	}
	node.UpLinkTunnel.PDR.PDI.UEIPAddress = &UEIPAddress{
		V4:          true,
		IPv4Address: pduAddress.To4(),
	}
	node.UpLinkTunnel.PDR.PDI.NetworkInstance = dnn

	node.UpLinkTunnel.PDR.OuterHeaderRemoval = &OuterHeaderRemoval{
		OuterHeaderRemovalDescription: OuterHeaderRemovalGtpUUdpIpv4,
	}

	node.UpLinkTunnel.PDR.FAR.ApplyAction = ApplyAction{
		Buff: false,
		Drop: false,
		Dupl: false,
		Forw: true,
		Nocp: false,
	}
	node.UpLinkTunnel.PDR.FAR.ForwardingParameters = &ForwardingParameters{
		DestinationInterface: DestinationInterface{
			InterfaceValue: DestinationInterfaceCore,
		},
		NetworkInstance: dnn,
	}

	node.UpLinkTunnel.PDR.FAR.ForwardingParameters.DestinationInterface.InterfaceValue = DestinationInterfaceSgiLanN6Lan
}

func (node *DataPathNode) ActivateDlLinkPdr(smContext *SMContext, pduAddress net.IP, defQER *QER, defURR *URR, defPrecedence uint32, dataPath *DataPath) {
	node.DownLinkTunnel.PDR.QER = defQER
	node.DownLinkTunnel.PDR.URR = defURR

	if node.DownLinkTunnel.PDR.Precedence == 0 {
		node.DownLinkTunnel.PDR.Precedence = defPrecedence
	}

	node.DownLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
	node.DownLinkTunnel.PDR.PDI.UEIPAddress = &UEIPAddress{
		V4:          true,
		IPv4Address: pduAddress.To4(),
	}

	if anIP := smContext.Tunnel.ANInformation.IPAddress; anIP != nil {
		dataPath.DPNode.DownLinkTunnel.PDR.FAR.ForwardingParameters = &ForwardingParameters{
			DestinationInterface: DestinationInterface{
				InterfaceValue: DestinationInterfaceAccess,
			},
			NetworkInstance: smContext.Dnn,
			OuterHeaderCreation: &OuterHeaderCreation{
				OuterHeaderCreationDescription: OuterHeaderCreationGtpUUdpIpv4,
				TeID:                           smContext.Tunnel.ANInformation.TEID,
				IPv4Address:                    anIP.To4(),
			},
		}
	}
}

func (dataPath *DataPath) ActivateTunnelAndPDR(smf *SMF, smContext *SMContext, smPolicyDecision *models.SmPolicyDecision, pduAddress net.IP, precedence uint32) error {
	smContext.AllocateLocalSEIDForDataPath(smf)

	err := dataPath.ActivateUlDlTunnel(smf)
	if err != nil {
		return fmt.Errorf("could not activate UL/DL Tunnel: %s", err)
	}

	defQER, err := dataPath.DPNode.CreateSessRuleQer(smf, smPolicyDecision)
	if err != nil {
		return fmt.Errorf("failed to create default QER: %v", err)
	}

	defULURR, err := smf.AddURR()
	if err != nil {
		return fmt.Errorf("failed to create uplink URR: %v", err)
	}

	defDLURR, err := smf.AddURR()
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

	if dataPath.DPNode.DownLinkTunnel != nil {
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.NetworkInstance = smContext.Dnn
		dataPath.DPNode.DownLinkTunnel.PDR.PDI.UEIPAddress = &UEIPAddress{
			V4:          true,
			IPv4Address: pduAddress.To4(),
		}
	}

	dataPath.Activated = true

	return nil
}

func (dataPath *DataPath) DeactivateTunnelAndPDR(smf *SMF) {
	dataPath.DPNode.DeactivateUpLinkTunnel(smf)
	dataPath.DPNode.DeactivateDownLinkTunnel(smf)

	dataPath.Activated = false
}

func BitRateTokbps(bitrate string) uint64 {
	s := strings.Split(bitrate, " ")

	digit, err := strconv.Atoi(s[0])
	if err != nil {
		return 0
	}

	switch s[1] {
	case "bps":
		return uint64(digit / 1000)
	case "Kbps":
		return uint64(digit * 1)
	case "Mbps":
		return uint64(digit * 1000)
	case "Gbps":
		return uint64(digit * 1000000)
	case "Tbps":
		return uint64(digit * 1000000000)
	default:
		return 0
	}
}
