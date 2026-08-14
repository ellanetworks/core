// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
)

// DownlinkState is what the downlink FAR does with a packet for the UE.
type DownlinkState uint8

const (
	// No access network is bound yet: a downlink packet has nowhere to go and
	// must not raise a notification, since no UE is idle to page.
	DownlinkDropping DownlinkState = iota
	DownlinkForwarding
	// CM/ECM-IDLE: buffer and notify so the UE is paged (TS 23.502 §4.2.3.3,
	// TS 23.401 §5.3.4.3).
	DownlinkBuffering
)

// dataPlane is everything the UPF needs to know about a session, and nothing
// derived from it. The rule set is computed from these facts by rules(), so the
// SMF holds no second copy of the UPF's state to keep in step.
type dataPlane struct {
	// The UE's addresses; UEIPv6 is the /64 base. An invalid Addr means that
	// family was not allocated.
	UEIPv4 netip.Addr
	UEIPv6 netip.Addr

	AN     AnchorBinding
	Access AccessType

	Downlink DownlinkState

	// The QoS the data plane enforces, which is not the signalled policy: a
	// network-requested modification programs the UPF before the UE answers, and
	// a reject or T3591 expiry leaves the enforced values in place
	// (TS 24.501 §6.3.2.5).
	QFI  uint8
	AMBR models.Ambr
}

// PFCP rule IDs are scoped to their PFCP session (TS 29.244 §5.2), and the UPF
// datapath keys every rule by SEID, so each session reuses the same fixed set.
// Fixed IDs need no cross-session allocator, which is why no rule ID can leak or
// be double-freed.
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

// rules is the session's complete rule set. It is total, so every request
// carries all of it and no call site decides which rules to send.
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
		// A zero F-TEID asks the UPF to allocate; the value comes back on the
		// establish response, and an update keeps the one already allocated.
		PDI: models.PDI{LocalFTEID: &models.FTEID{}},
	}}

	// A PDR matches one UE address, so a dual-stack session needs a second
	// downlink PDR. Both name the one downlink FAR: same forwarding, and the UPF
	// must not be sent that rule twice.
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

// forwardingParameters encapsulates towards the access network endpoint. It is
// carried whatever the downlink is doing, so a session that returns to
// forwarding needs only the apply-action to change.
func (d dataPlane) forwardingParameters() *models.ForwardingParameters {
	// 4G S1-U carries no PDU Session Container (TS 38.415: that is N3/N9-only).
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

// A URR is created with the session and removed with it, so a modification
// carries every rule but those.
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
