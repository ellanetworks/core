// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net"
	"net/netip"

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
	SecondPDR      *PDR
	Activated      bool
}

func (dp *DataPath) ActivateUpLinkPdr(ueIP netip.Addr, anIP net.IP, defQER *QER, defURR *URR) {
	dp.UpLinkTunnel.PDR.QER = defQER
	dp.UpLinkTunnel.PDR.URR = defURR

	dp.UpLinkTunnel.PDR.PDI.LocalFTEID = &models.FTEID{}
	dp.UpLinkTunnel.PDR.PDI.UEIPAddress = ueIP

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

func (dp *DataPath) ActivateDlLinkPdr(anIPv4 net.IP, anIPv6 net.IP, teid uint32, ueIP netip.Addr, defQER *QER, defURR *URR) {
	dp.DownLinkTunnel.PDR.QER = defQER
	dp.DownLinkTunnel.PDR.URR = defURR

	dp.DownLinkTunnel.PDR.PDI.UEIPAddress = ueIP

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

func (dp *DataPath) ActivateTunnelAndPDR(smf *SMF, smContext *SMContext, policy *Policy, ueIP netip.Addr) error {
	seid := smf.AllocateLocalSEID()

	smContext.SetPFCPSession(seid)

	dp.UpLinkTunnel.PDR = NewPDR(pdrIDUplink, farIDUplink)
	dp.DownLinkTunnel.PDR = NewPDR(pdrIDDownlink, farIDDownlink)

	defQER := NewQER(policy, qerIDDefault)
	defULURR := newURR(urrIDUplink)
	defDLURR := newURR(urrIDDownlink)

	dp.ActivateUpLinkPdr(ueIP, smContext.Tunnel.ANInformation.IPv4Address, defQER, defULURR)

	dp.ActivateDlLinkPdr(smContext.Tunnel.ANInformation.IPv4Address, smContext.Tunnel.ANInformation.IPv6Address, smContext.Tunnel.ANInformation.TEID, ueIP, defQER, defDLURR)

	if smContext.PDUIPV4Address != nil && smContext.PDUIPV6Prefix != nil {
		secondPdr := &PDR{PDRID: pdrIDSecond}
		secondPdr.FAR = dp.DownLinkTunnel.PDR.FAR
		secondPdr.QER = defQER
		secondPdr.URR = defDLURR
		secondPdr.PDI.UEIPAddress, _ = netip.AddrFromSlice(smContext.PDUIPV6Prefix.To16())

		dp.SecondPDR = secondPdr
	}

	dp.Activated = true

	return nil
}

// DeactivateTunnelAndPDR resets the data path. Safe to call more than once.
func (dp *DataPath) DeactivateTunnelAndPDR() {
	dp.UpLinkTunnel = &GTPTunnel{}
	dp.DownLinkTunnel = &GTPTunnel{}
	dp.SecondPDR = nil
	dp.Activated = false
}
