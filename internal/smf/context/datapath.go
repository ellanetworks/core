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

type GTPTunnel struct {
	PDR  *PDR
	TEID uint32
	N3IP net.IP
}

type DataPath struct {
	UpLinkTunnel   *GTPTunnel
	DownLinkTunnel *GTPTunnel
	Activated      bool
}

func (dp *DataPath) ActivateUpLinkTunnel(smf *SMF) error {
	pdr, err := smf.NewPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	dp.UpLinkTunnel.PDR = pdr

	return nil
}

func (dp *DataPath) ActivateDownLinkTunnel(smf *SMF) error {
	pdr, err := smf.NewPDR()
	if err != nil {
		return fmt.Errorf("add PDR failed: %s", err)
	}

	dp.DownLinkTunnel.PDR = pdr

	return nil
}

func (dp *DataPath) DeactivateUpLinkTunnel(smf *SMF) {
	if dp.UpLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in UpLink Tunnel")
		return
	}

	smf.RemovePDR(dp.UpLinkTunnel.PDR)

	if far := dp.UpLinkTunnel.PDR.FAR; far != nil {
		smf.RemoveFAR(far)
	}

	qer := dp.UpLinkTunnel.PDR.QER
	if qer != nil {
		smf.RemoveQER(qer)
	}

	urr := dp.UpLinkTunnel.PDR.URR
	if urr != nil {
		smf.RemoveURR(urr)
	}

	logger.SmfLog.Info("deactivated UpLinkTunnel PDR ")

	dp.DownLinkTunnel = &GTPTunnel{}
}

func (dp *DataPath) DeactivateDownLinkTunnel(smf *SMF) {
	if dp.DownLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in Downlink Tunnel")
		return
	}

	logger.SmfLog.Info("deactivated DownLinkTunnel PDR", zap.Uint16("pdrId", dp.DownLinkTunnel.PDR.PDRID))

	smf.RemovePDR(dp.DownLinkTunnel.PDR)

	if far := dp.DownLinkTunnel.PDR.FAR; far != nil {
		smf.RemoveFAR(far)
	}

	qer := dp.DownLinkTunnel.PDR.QER
	if qer != nil {
		smf.RemoveQER(qer)
	}

	urr := dp.DownLinkTunnel.PDR.URR
	if urr != nil {
		smf.RemoveURR(urr)
	}

	dp.DownLinkTunnel = &GTPTunnel{}
}

func (dataPath *DataPath) ActivateUlDlTunnel(smf *SMF) error {
	err := dataPath.ActivateUpLinkTunnel(smf)
	if err != nil {
		return fmt.Errorf("couldn't activate UpLinkTunnel: %s", err)
	}

	err = dataPath.ActivateDownLinkTunnel(smf)
	if err != nil {
		return fmt.Errorf("couldn't activate DownLinkTunnel: %s", err)
	}

	return nil
}

func (dp *DataPath) ActivateUpLinkPdr(dnn string, pduAddress net.IP, defQER *QER, defURR *URR, defPrecedence uint32) {
	dp.UpLinkTunnel.PDR.QER = defQER
	dp.UpLinkTunnel.PDR.URR = defURR

	// Set Default precedence
	if dp.UpLinkTunnel.PDR.Precedence == 0 {
		dp.UpLinkTunnel.PDR.Precedence = defPrecedence
	}

	dp.UpLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceAccess}
	dp.UpLinkTunnel.PDR.PDI.LocalFTeID = &FTEID{
		Ch: true,
	}
	dp.UpLinkTunnel.PDR.PDI.UEIPAddress = &UEIPAddress{
		V4:          true,
		IPv4Address: pduAddress.To4(),
	}
	dp.UpLinkTunnel.PDR.PDI.NetworkInstance = dnn
	dp.UpLinkTunnel.PDR.OuterHeaderRemoval = &OuterHeaderRemoval{
		OuterHeaderRemovalDescription: OuterHeaderRemovalGtpUUdpIpv4,
	}

	dp.UpLinkTunnel.PDR.FAR.ApplyAction = ApplyAction{
		Buff: false,
		Drop: false,
		Dupl: false,
		Forw: true,
		Nocp: false,
	}
	dp.UpLinkTunnel.PDR.FAR.ForwardingParameters = &ForwardingParameters{
		DestinationInterface: DestinationInterface{
			InterfaceValue: DestinationInterfaceCore,
		},
		NetworkInstance: dnn,
	}

	dp.UpLinkTunnel.PDR.FAR.ForwardingParameters.DestinationInterface.InterfaceValue = DestinationInterfaceSgiLanN6Lan
}

func (dp *DataPath) ActivateDlLinkPdr(smContext *SMContext, pduAddress net.IP, defQER *QER, defURR *URR, defPrecedence uint32) {
	dp.DownLinkTunnel.PDR.QER = defQER
	dp.DownLinkTunnel.PDR.URR = defURR

	if dp.DownLinkTunnel.PDR.Precedence == 0 {
		dp.DownLinkTunnel.PDR.Precedence = defPrecedence
	}

	dp.DownLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
	dp.DownLinkTunnel.PDR.PDI.UEIPAddress = &UEIPAddress{
		V4:          true,
		IPv4Address: pduAddress.To4(),
	}

	if anIP := smContext.Tunnel.ANInformation.IPAddress; anIP != nil {
		dp.DownLinkTunnel.PDR.FAR.ForwardingParameters = &ForwardingParameters{
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

	defQER, err := smf.NewQER(smPolicyDecision)
	if err != nil {
		return fmt.Errorf("failed to create default QER: %v", err)
	}

	defULURR, err := smf.NewURR()
	if err != nil {
		return fmt.Errorf("failed to create uplink URR: %v", err)
	}

	defDLURR, err := smf.NewURR()
	if err != nil {
		return fmt.Errorf("failed to create uplink URR: %v", err)
	}

	// Setup UpLink PDR
	if dataPath.UpLinkTunnel != nil {
		dataPath.ActivateUpLinkPdr(smContext.Dnn, pduAddress, defQER, defULURR, precedence)
	}

	// Setup DownLink PDR
	if dataPath.DownLinkTunnel != nil {
		dataPath.ActivateDlLinkPdr(smContext, pduAddress, defQER, defDLURR, precedence)
	}

	if dataPath.DownLinkTunnel != nil {
		dataPath.DownLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
		dataPath.DownLinkTunnel.PDR.PDI.NetworkInstance = smContext.Dnn
		dataPath.DownLinkTunnel.PDR.PDI.UEIPAddress = &UEIPAddress{
			V4:          true,
			IPv4Address: pduAddress.To4(),
		}
	}

	dataPath.Activated = true

	return nil
}

func (dataPath *DataPath) DeactivateTunnelAndPDR(smf *SMF) {
	dataPath.DeactivateUpLinkTunnel(smf)
	dataPath.DeactivateDownLinkTunnel(smf)

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
