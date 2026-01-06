// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net"

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

func (dp *DataPath) DeactivateUpLinkTunnel(smf *SMF) {
	if dp.UpLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in UpLink Tunnel")
		return
	}

	smf.RemovePDR(dp.UpLinkTunnel.PDR)

	if dp.UpLinkTunnel.PDR.FAR != nil {
		smf.RemoveFAR(dp.UpLinkTunnel.PDR.FAR)
	}

	if dp.UpLinkTunnel.PDR.QER != nil {
		smf.RemoveQER(dp.UpLinkTunnel.PDR.QER)
	}

	if dp.UpLinkTunnel.PDR.URR != nil {
		smf.RemoveURR(dp.UpLinkTunnel.PDR.URR)
	}

	logger.SmfLog.Info("deactivated UpLinkTunnel PDR")

	dp.UpLinkTunnel = &GTPTunnel{}
}

func (dp *DataPath) DeactivateDownLinkTunnel(smf *SMF) {
	if dp.DownLinkTunnel.PDR == nil {
		logger.SmfLog.Debug("PDR is nil in Downlink Tunnel")
		return
	}

	logger.SmfLog.Info("deactivated DownLinkTunnel PDR", zap.Uint16("pdrId", dp.DownLinkTunnel.PDR.PDRID))

	smf.RemovePDR(dp.DownLinkTunnel.PDR)

	if dp.DownLinkTunnel.PDR.FAR != nil {
		smf.RemoveFAR(dp.DownLinkTunnel.PDR.FAR)
	}

	if dp.DownLinkTunnel.PDR.QER != nil {
		smf.RemoveQER(dp.DownLinkTunnel.PDR.QER)
	}

	if dp.DownLinkTunnel.PDR.URR != nil {
		smf.RemoveURR(dp.DownLinkTunnel.PDR.URR)
	}

	dp.DownLinkTunnel = &GTPTunnel{}
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

func (dp *DataPath) ActivateDlLinkPdr(dnn string, anIP net.IP, teid uint32, pduAddress net.IP, defQER *QER, defURR *URR, defPrecedence uint32) {
	dp.DownLinkTunnel.PDR.QER = defQER
	dp.DownLinkTunnel.PDR.URR = defURR

	if dp.DownLinkTunnel.PDR.Precedence == 0 {
		dp.DownLinkTunnel.PDR.Precedence = defPrecedence
	}

	dp.DownLinkTunnel.PDR.PDI.SourceInterface = SourceInterface{InterfaceValue: SourceInterfaceCore}
	dp.DownLinkTunnel.PDR.PDI.NetworkInstance = dnn
	dp.DownLinkTunnel.PDR.PDI.UEIPAddress = &UEIPAddress{
		V4:          true,
		IPv4Address: pduAddress.To4(),
	}

	if anIP != nil {
		dp.DownLinkTunnel.PDR.FAR.ForwardingParameters = &ForwardingParameters{
			DestinationInterface: DestinationInterface{
				InterfaceValue: DestinationInterfaceAccess,
			},
			NetworkInstance: dnn,
			OuterHeaderCreation: &OuterHeaderCreation{
				OuterHeaderCreationDescription: OuterHeaderCreationGtpUUdpIpv4,
				TeID:                           teid,
				IPv4Address:                    anIP.To4(),
			},
		}
	}
}

func (dp *DataPath) ActivateTunnelAndPDR(smf *SMF, smContext *SMContext, smPolicyDecision *models.SmPolicyData, pduAddress net.IP, precedence uint32) error {
	smContext.AllocateLocalSEIDForDataPath(smf)

	ulPdr, err := smf.NewPDR()
	if err != nil {
		return fmt.Errorf("could not create uplink PDR: %s", err)
	}

	dp.UpLinkTunnel.PDR = ulPdr

	dlPdr, err := smf.NewPDR()
	if err != nil {
		return fmt.Errorf("could not create downlink PDR: %s", err)
	}

	dp.DownLinkTunnel.PDR = dlPdr

	defQER, err := smf.NewQER(smPolicyDecision)
	if err != nil {
		return fmt.Errorf("could not create QER: %v", err)
	}

	defULURR, err := smf.NewURR()
	if err != nil {
		return fmt.Errorf("could not create uplink URR: %v", err)
	}

	defDLURR, err := smf.NewURR()
	if err != nil {
		return fmt.Errorf("could not create downlink URR: %v", err)
	}

	dp.ActivateUpLinkPdr(smContext.Dnn, pduAddress, defQER, defULURR, precedence)

	dp.ActivateDlLinkPdr(smContext.Dnn, smContext.Tunnel.ANInformation.IPAddress, smContext.Tunnel.ANInformation.TEID, pduAddress, defQER, defDLURR, precedence)

	dp.Activated = true

	return nil
}

func (dp *DataPath) DeactivateTunnelAndPDR(smf *SMF) {
	dp.DeactivateUpLinkTunnel(smf)
	dp.DeactivateDownLinkTunnel(smf)

	dp.Activated = false
}
