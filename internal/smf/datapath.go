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

func (t *UPTunnel) Activate(policy *Policy, ueIP netip.Addr, ueIPv6Prefix net.IP) {
	t.UplinkPDR = NewPDR(pdrIDUplink, farIDUplink)
	t.DownlinkPDR = NewPDR(pdrIDDownlink, farIDDownlink)
	t.QER = NewQER(policy, qerIDDefault)
	t.UplinkURR = newURR(urrIDUplink)
	t.DownlinkURR = newURR(urrIDDownlink)

	t.UplinkPDR.QER, t.UplinkPDR.URR = t.QER, t.UplinkURR
	t.DownlinkPDR.QER, t.DownlinkPDR.URR = t.QER, t.DownlinkURR

	t.activateUplinkPDR(ueIP)
	t.activateDownlinkPDR(ueIP)

	// A PDR matches one UE address, so a dual-stack session needs a second
	// downlink PDR. It shares the downlink FAR: same forwarding, and the UPF must
	// not be sent the rule twice.
	if ueIPv6Prefix != nil {
		prefix, _ := netip.AddrFromSlice(ueIPv6Prefix.To16())

		t.DownlinkPDRv6 = &PDR{
			PDRID: pdrIDSecond,
			FAR:   t.DownlinkPDR.FAR,
			QER:   t.QER,
			URR:   t.DownlinkURR,
		}
		t.DownlinkPDRv6.PDI.UEIPAddress = prefix
	}

	t.Activated = true
}

func (t *UPTunnel) activateUplinkPDR(ueIP netip.Addr) {
	pdr := t.UplinkPDR

	// A zero F-TEID asks the UPF to allocate; the value comes back on the
	// establish response.
	pdr.PDI.LocalFTEID = &models.FTEID{}
	pdr.PDI.UEIPAddress = ueIP

	ohr := models.OuterHeaderRemovalGtpUUdpIpv4
	if t.AN.IPv4 != nil && t.AN.IPv4.To4() == nil {
		ohr = models.OuterHeaderRemovalGtpUUdpIpv6
	}

	pdr.OuterHeaderRemoval = &ohr

	pdr.FAR.ApplyAction = models.ApplyAction{Forw: true}
	pdr.FAR.ForwardingParameters = &models.ForwardingParameters{}
}

func (t *UPTunnel) activateDownlinkPDR(ueIP netip.Addr) {
	pdr := t.DownlinkPDR

	pdr.PDI.UEIPAddress = ueIP

	switch {
	case t.AN.IPv6 != nil:
		pdr.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv6,
				TEID:        t.AN.TEID,
				IPv6Address: t.AN.IPv6,
			},
		}
	case t.AN.IPv4 != nil:
		pdr.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        t.AN.TEID,
				IPv4Address: t.AN.IPv4.To4(),
			},
		}
	}
}

func (t *UPTunnel) PDRs() []*PDR {
	if !t.Activated {
		return nil
	}

	pdrs := []*PDR{t.UplinkPDR, t.DownlinkPDR}
	if t.DownlinkPDRv6 != nil {
		pdrs = append(pdrs, t.DownlinkPDRv6)
	}

	return pdrs
}

// The two downlink PDRs share one FAR, which the UPF must be sent once.
func (t *UPTunnel) FARs() []*FAR {
	if !t.Activated {
		return nil
	}

	return []*FAR{t.UplinkPDR.FAR, t.DownlinkPDR.FAR}
}

func (t *UPTunnel) QERs() []*QER {
	if !t.Activated {
		return nil
	}

	return []*QER{t.QER}
}

func (t *UPTunnel) URRs() []*URR {
	if !t.Activated {
		return nil
	}

	return []*URR{t.UplinkURR, t.DownlinkURR}
}
