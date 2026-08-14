// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
)

type DownlinkState uint8

const (
	DownlinkDropping DownlinkState = iota
	DownlinkForwarding
	DownlinkBuffering
)

type dataPlane struct {
	UEIPv4   netip.Addr
	UEIPv6   netip.Addr
	AN       AnchorBinding
	Access   AccessType
	Downlink DownlinkState
	QFI      uint8
	AMBR     models.Ambr
}

const (
	pdrIDUplink   uint16 = 1
	pdrIDDownlink uint16 = 2
	pdrIDSecond   uint16 = 3

	farIDUplink   uint32 = 1
	farIDDownlink uint32 = 2

	qerIDDefault uint32 = 1

	urrIDUplink   uint32 = 1
	urrIDDownlink uint32 = 2
)

func (d dataPlane) valid() error {
	if d.Downlink == DownlinkForwarding && !d.AN.bound() {
		return fmt.Errorf("the downlink cannot forward with no access-network endpoint")
	}

	return nil
}

func (d dataPlane) rules() (pdrs []models.PDR, fars []models.FAR, qers []models.QER, urrs []models.URR) {
	ohr := models.OuterHeaderRemovalGtpUUdpIpv4
	if d.AN.IPv6 != nil {
		ohr = models.OuterHeaderRemovalGtpUUdpIpv6
	}

	pdrs = []models.PDR{{
		PDRID:              pdrIDUplink,
		OuterHeaderRemoval: &ohr,
		FARID:              farIDUplink,
		QERID:              qerIDDefault,
		URRID:              urrIDUplink,
		PDI:                models.PDI{LocalFTEID: &models.FTEID{}},
	}}

	if d.UEIPv4.IsValid() {
		pdrs = append(pdrs, downlinkPDR(pdrIDDownlink, d.UEIPv4))
	}

	if d.UEIPv6.IsValid() {
		id := pdrIDDownlink
		if d.UEIPv4.IsValid() {
			id = pdrIDSecond
		}

		pdrs = append(pdrs, downlinkPDR(id, d.UEIPv6))
	}

	fars = []models.FAR{
		{
			FARID:                farIDUplink,
			ApplyAction:          models.ApplyAction{Forw: true},
			ForwardingParameters: &models.ForwardingParameters{},
		},
		{
			FARID:                farIDDownlink,
			ApplyAction:          d.Downlink.applyAction(),
			ForwardingParameters: d.forwardingParameters(),
		},
	}

	qers = []models.QER{{
		QERID: qerIDDefault,
		QFI:   d.QFI,
		GateStatus: &models.GateStatus{
			ULGate: models.GateOpen,
			DLGate: models.GateOpen,
		},
		MBR: &models.MBR{
			ULMBR: d.AMBR.Uplink.Kbps(),
			DLMBR: d.AMBR.Downlink.Kbps(),
		},
	}}

	urrs = []models.URR{{URRID: urrIDUplink}, {URRID: urrIDDownlink}}

	return pdrs, fars, qers, urrs
}

func downlinkPDR(pdrID uint16, ueIP netip.Addr) models.PDR {
	return models.PDR{
		PDRID: pdrID,
		FARID: farIDDownlink,
		QERID: qerIDDefault,
		URRID: urrIDDownlink,
		PDI:   models.PDI{UEIPAddress: ueIP},
	}
}

func (s DownlinkState) applyAction() models.ApplyAction {
	switch s {
	case DownlinkForwarding:
		return models.ApplyAction{Forw: true}
	case DownlinkBuffering:
		return models.ApplyAction{Buff: true, Nocp: true}
	default:
		return models.ApplyAction{Drop: true}
	}
}

func (d dataPlane) forwardingParameters() *models.ForwardingParameters {
	s1u := d.Access == Access4G

	switch {
	case d.AN.IPv6 != nil:
		return &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv6,
				TEID:        d.AN.TEID,
				IPv6Address: d.AN.IPv6,
				S1U:         s1u,
			},
		}
	case d.AN.IPv4 != nil:
		return &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        d.AN.TEID,
				IPv4Address: d.AN.IPv4.To4(),
				S1U:         s1u,
			},
		}
	default:
		return &models.ForwardingParameters{}
	}
}

func (d dataPlane) establishRequest(seid uint64, imsi, policyID string, framedRoutes []netip.Prefix) *models.EstablishRequest {
	pdrs, fars, qers, urrs := d.rules()

	return &models.EstablishRequest{
		SEID:         seid,
		IMSI:         imsi,
		PolicyID:     policyID,
		PDRs:         pdrs,
		FARs:         fars,
		QERs:         qers,
		URRs:         urrs,
		FramedRoutes: framedRoutes,
	}
}

func (d dataPlane) modifyRequest(seid uint64, policyID string) *models.ModifyRequest {
	pdrs, fars, qers, _ := d.rules()

	return &models.ModifyRequest{
		SEID:       seid,
		PolicyID:   policyID,
		UpdatePDRs: pdrs,
		UpdateFARs: fars,
		UpdateQERs: qers,
	}
}
