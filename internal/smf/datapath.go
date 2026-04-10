// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

type GTPTunnel struct {
	PDR    *PDR
	TEID   uint32
	N3IPv4 netip.Addr
	N3IPv6 netip.Addr
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

	logger.SmfLog.Info("deactivated DownLinkTunnel PDR", logger.PDRID(uint32(dp.DownLinkTunnel.PDR.PDRID)))

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

func (dp *DataPath) ActivateUpLinkPdr(pduAddress net.IP, anIP net.IP, defQER *QER, defURR *URR) {
	dp.UpLinkTunnel.PDR.QER = defQER
	dp.UpLinkTunnel.PDR.URR = defURR

	dp.UpLinkTunnel.PDR.PDI.LocalFTEID = &models.FTEID{}
	dp.UpLinkTunnel.PDR.PDI.UEIPAddress = netip.AddrFrom4([4]byte(pduAddress.To4()))

	ohr := models.OuterHeaderRemovalGtpUUdpIpv4
	if anIP != nil && anIP.To4() == nil {
		ohr = models.OuterHeaderRemovalGtpUUdpIpv6
	}

	dp.UpLinkTunnel.PDR.OuterHeaderRemoval = &ohr

	dp.UpLinkTunnel.PDR.FAR.ApplyAction = models.ApplyAction{
		Forw: true,
	}
	dp.UpLinkTunnel.PDR.FAR.ForwardingParameters = &models.ForwardingParameters{}
}

func (dp *DataPath) ActivateDlLinkPdr(anIPv4 net.IP, anIPv6 net.IP, teid uint32, pduAddress net.IP, defQER *QER, defURR *URR) {
	dp.DownLinkTunnel.PDR.QER = defQER
	dp.DownLinkTunnel.PDR.URR = defURR

	dp.DownLinkTunnel.PDR.PDI.UEIPAddress = netip.AddrFrom4([4]byte(pduAddress.To4()))

	// When both addresses are available, IPv6 is preferred for the downlink tunnel.
	// This preference is intentional and is documented in the configuration file reference.
	if anIPv6 != nil {
		dp.DownLinkTunnel.PDR.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv6,
				TEID:        teid,
				IPv6Address: anIPv6,
			},
		}
	} else if anIPv4 != nil {
		dp.DownLinkTunnel.PDR.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        teid,
				IPv4Address: anIPv4.To4(),
			},
		}
	}
}

func (dp *DataPath) ActivateTunnelAndPDR(smf *SMF, smContext *SMContext, policy *Policy, pduAddress net.IP) error {
	seid := smf.AllocateLocalSEID()

	smContext.SetPFCPSession(seid)

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

	defQER, err := smf.NewQER(policy)
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

	dp.ActivateUpLinkPdr(pduAddress, smContext.Tunnel.ANInformation.IPAddress, defQER, defULURR)

	dp.ActivateDlLinkPdr(smContext.Tunnel.ANInformation.IPAddress, smContext.Tunnel.ANInformation.IPv6Address, smContext.Tunnel.ANInformation.TEID, pduAddress, defQER, defDLURR)

	dp.Activated = true

	return nil
}

func (dp *DataPath) DeactivateTunnelAndPDR(smf *SMF) {
	dp.DeactivateUpLinkTunnel(smf)
	dp.DeactivateDownLinkTunnel(smf)

	dp.Activated = false
}
