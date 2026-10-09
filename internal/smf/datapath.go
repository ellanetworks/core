// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"cmp"
	"fmt"
	"net/netip"
	"slices"
	"time"

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

	Forwarding *AnchorBinding
	Bearers    []bearerLeg
}

type bearerLeg struct {
	Slot         uint8
	QFI          uint8
	Admitted     bool
	TargetUplink bool
	Rules        []ruleLeg
	AN           AnchorBinding
	Forwarding   *AnchorBinding
}

type ruleLeg struct {
	Index   uint8
	Filters []models.SDFFilter
	MBR     models.Ambr
	Gate    models.GateStatus
}

const (
	pdrIDUplink     uint16 = 1
	pdrIDDownlink   uint16 = 2
	pdrIDSecond     uint16 = 3
	pdrIDForwarding uint16 = 4

	farIDUplink     uint32 = 1
	farIDDownlink   uint32 = 2
	farIDForwarding uint32 = 3

	qerIDDefault uint32 = 1

	urrIDUplink   uint32 = 1
	urrIDDownlink uint32 = 2

	pdrIDBearerForwardingBase uint16 = 64
	pdrIDRuleBase             uint16 = 256
	farIDBearerBase           uint32 = 16
	farIDBearerForwardingBase uint32 = 32
	qerIDRuleBase             uint32 = 256

	chooseIDSession              uint8  = 1
	chooseIDForwarding           uint8  = 4
	chooseIDBearerBase           uint8  = 16
	chooseIDBearerForwardingBase uint8  = 32
	rulePDRsPerRule              uint16 = 4
	rulePDRTargetUplink          uint16 = 3
	rulePDRsPerBearer                   = rulePDRsPerRule * maxDedicatedFilters
	pdrPrecedenceDefault         uint32 = 256

	bearerAveragingWindow = 2000 * time.Millisecond
)

func pdrIDRule(slot, index uint8, leg uint16) uint16 {
	return pdrIDRuleBase + uint16(slot)*rulePDRsPerBearer + uint16(index)*rulePDRsPerRule + leg
}

func ruleOfPDR(id uint16) (slot, index uint8, leg uint16, ok bool) {
	if id < pdrIDRuleBase || id >= pdrIDRuleBase+maxDedicatedBearers*rulePDRsPerBearer {
		return 0, 0, 0, false
	}

	off := id - pdrIDRuleBase

	return uint8(off / rulePDRsPerBearer), uint8(off % rulePDRsPerBearer / rulePDRsPerRule), off % rulePDRsPerRule, true
}

func qerIDRule(slot, index uint8) uint32 {
	return qerIDRuleBase + uint32(slot)*maxDedicatedFilters + uint32(index)
}

func farIDBearer(slot uint8) uint32 { return farIDBearerBase + uint32(slot) }

func bearerDownlinkPDROfFAR(id uint32) uint16 {
	if id < farIDBearerBase || id >= farIDBearerBase+maxDedicatedBearers {
		return 0
	}

	return pdrIDRule(uint8(id-farIDBearerBase), 0, 1)
}

func pdrIDBearerForwarding(slot uint8) uint16 { return pdrIDBearerForwardingBase + uint16(slot) }

func farIDBearerForwarding(slot uint8) uint32 { return farIDBearerForwardingBase + uint32(slot) }

func chooseIDBearer(slot uint8) uint8 { return chooseIDBearerBase + slot }

func chooseIDBearerForwarding(slot uint8) uint8 { return chooseIDBearerForwardingBase + slot }

func directionFilters(filters []models.SDFFilter, dir models.FilterDirection) []models.SDFFilter {
	var out []models.SDFFilter

	for _, f := range filters {
		if f.Direction == dir || f.Direction == models.FilterBidirectional {
			out = append(out, f)
		}
	}

	return out
}

func filtersPrecedence(filters []models.SDFFilter) uint32 {
	if len(filters) == 0 {
		return pdrPrecedenceDefault
	}

	return uint32(slices.MinFunc(filters, func(a, b models.SDFFilter) int { return cmp.Compare(a.Precedence, b.Precedence) }).Precedence)
}

func (d dataPlane) ueAddresses() []netip.Addr {
	var out []netip.Addr

	for _, a := range []netip.Addr{d.UEIPv4, d.UEIPv6} {
		if a.IsValid() {
			out = append(out, a)
		}
	}

	return out
}

func (d dataPlane) checkSDFCapacity() error {
	ues := d.ueAddresses()

	var rules, pdrs int

	for _, b := range d.Bearers {
		for _, r := range b.Rules {
			uplink := directionFilters(r.Filters, models.FilterUplink)
			rules += len(uplink)

			if len(uplink) > 0 {
				pdrs++
			}

			if b.TargetUplink && len(uplink) > 0 {
				rules += len(uplink)
				pdrs++
			}

			downlink := directionFilters(r.Filters, models.FilterDownlink)

			for _, ue := range ues {
				matching := slices.DeleteFunc(slices.Clone(downlink), func(f models.SDFFilter) bool { return !sameFamily(f, ue) })
				if len(matching) > 0 {
					rules += len(matching)
					pdrs++
				}
			}
		}
	}

	if rules > models.MaxSessionSDFRules || pdrs > models.MaxSessionSDFPDRs {
		return fmt.Errorf("the session's bearers need %d SDF filters in %d PDRs, the user plane holds %d in %d", rules, pdrs, models.MaxSessionSDFRules, models.MaxSessionSDFPDRs)
	}

	return nil
}

func (d dataPlane) accessSwitchedFrom(prev dataPlane) bool {
	if d.AN.switchedFrom(prev.AN) {
		return true
	}

	return slices.ContainsFunc(d.Bearers, func(l bearerLeg) bool {
		i := slices.IndexFunc(prev.Bearers, func(p bearerLeg) bool { return p.Slot == l.Slot })
		return i >= 0 && l.AN.switchedFrom(prev.Bearers[i].AN)
	})
}

func (r ruleLeg) policedGate() models.GateStatus {
	g := r.Gate

	if r.MBR.Uplink.IsZero() {
		g.ULGate = models.GateClose
	}

	if r.MBR.Downlink.IsZero() {
		g.DLGate = models.GateClose
	}

	return g
}

func ceilKbps(b models.BitRate) uint64 {
	return (b.Bps() + 999) / 1000
}

func sameFamily(f models.SDFFilter, ue netip.Addr) bool {
	return !f.Remote.IsValid() || f.Remote.Addr().Is4() == ue.Is4()
}

func (d dataPlane) uplinkQFI() uint8 {
	if d.Access == Access5G {
		return d.QFI
	}

	return 0
}

func (d dataPlane) bearerRules(b bearerLeg) (pdrs []models.PDR, fars []models.FAR, qers []models.QER) {
	ohr := models.OuterHeaderRemovalGtpUUdpIP
	downlinkProgrammed := d.Downlink != DownlinkDropping && (d.Downlink != DownlinkForwarding || b.AN.bound())
	chooseID, downlinkFAR, qfi := chooseIDBearer(b.Slot), farIDBearer(b.Slot), uint8(0)

	if d.Access == Access5G {
		downlinkProgrammed = d.Downlink != DownlinkDropping && b.Admitted
		chooseID, downlinkFAR, qfi = chooseIDSession, farIDDownlink, b.QFI
	}

	for _, r := range b.Rules {
		qerID := qerIDRule(b.Slot, r.Index)

		qers = append(qers, models.QER{
			QERID:           qerID,
			QFI:             qfi,
			GateStatus:      new(r.policedGate()),
			MBR:             &models.MBR{ULMBR: ceilKbps(r.MBR.Uplink), DLMBR: ceilKbps(r.MBR.Downlink)},
			AveragingWindow: new(bearerAveragingWindow),
		})

		if uplink := directionFilters(r.Filters, models.FilterUplink); len(uplink) > 0 {
			pdrs = append(pdrs, models.PDR{
				PDRID:              pdrIDRule(b.Slot, r.Index, 0),
				Precedence:         filtersPrecedence(uplink),
				OuterHeaderRemoval: &ohr,
				FARID:              farIDUplink,
				QERID:              qerID,
				URRID:              urrIDUplink,
				PDI: models.PDI{
					SourceInterface: models.InterfaceAccess,
					LocalFTEID:      &models.FTEID{ChooseID: chooseID},
					QFI:             qfi,
					SDFFilters:      uplink,
				},
			})

			if d.Access == Access5G && b.TargetUplink {
				pdrs = append(pdrs, models.PDR{
					PDRID:              pdrIDRule(b.Slot, r.Index, rulePDRTargetUplink),
					Precedence:         filtersPrecedence(uplink),
					OuterHeaderRemoval: &ohr,
					FARID:              farIDUplink,
					QERID:              qerID,
					URRID:              urrIDUplink,
					PDI: models.PDI{
						SourceInterface: models.InterfaceAccess,
						LocalFTEID:      &models.FTEID{ChooseID: chooseIDBearer(b.Slot)},
						SDFFilters:      uplink,
					},
				})
			}
		}

		if !downlinkProgrammed {
			continue
		}

		downlink := directionFilters(r.Filters, models.FilterDownlink)

		for i, ue := range d.ueAddresses() {
			matching := slices.DeleteFunc(slices.Clone(downlink), func(f models.SDFFilter) bool { return !sameFamily(f, ue) })
			if len(matching) == 0 {
				continue
			}

			pdrs = append(pdrs, models.PDR{
				PDRID:      pdrIDRule(b.Slot, r.Index, uint16(i)+1),
				Precedence: filtersPrecedence(matching),
				FARID:      downlinkFAR,
				QERID:      qerID,
				URRID:      urrIDDownlink,
				PDI: models.PDI{
					SourceInterface: models.InterfaceCore,
					UEIPAddress:     ue,
					SDFFilters:      matching,
				},
			})
		}
	}

	if b.Forwarding != nil && d.Access != Access5G {
		pdrs = append(pdrs, models.PDR{
			PDRID:              pdrIDBearerForwarding(b.Slot),
			Precedence:         pdrPrecedenceDefault,
			OuterHeaderRemoval: &ohr,
			FARID:              farIDBearerForwarding(b.Slot),
			PDI: models.PDI{
				SourceInterface: models.InterfaceAccess,
				LocalFTEID:      &models.FTEID{ChooseID: chooseIDBearerForwarding(b.Slot)},
			},
		})

		fars = append(fars, models.FAR{
			FARID:                farIDBearerForwarding(b.Slot),
			ApplyAction:          models.ApplyAction{Forw: true},
			ForwardingParameters: forwardingTunnelParameters(*b.Forwarding, d.Access),
		})
	}

	if downlinkProgrammed && slices.ContainsFunc(pdrs, func(p models.PDR) bool { return p.FARID == farIDBearer(b.Slot) }) {
		fars = append(fars, models.FAR{
			FARID:                farIDBearer(b.Slot),
			ApplyAction:          d.bearerDownlinkAction(),
			ForwardingParameters: accessForwardingParameters(b.AN, d.Access),
		})
	}

	return pdrs, fars, qers
}

func (d dataPlane) bearerDownlinkAction() models.ApplyAction {
	if d.Downlink == DownlinkBuffering {
		return models.ApplyAction{Buff: true, Nocp: true}
	}

	return models.ApplyAction{Forw: true}
}

func (d dataPlane) bearerOfDownlinkPDR(id uint16) (uint8, bool) {
	slot, _, leg, ok := ruleOfPDR(id)
	if !ok || leg == 0 || leg == rulePDRTargetUplink {
		return 0, false
	}

	return slot, slices.ContainsFunc(d.Bearers, func(b bearerLeg) bool { return b.Slot == slot })
}

func (d dataPlane) valid() error {
	if d.Downlink == DownlinkForwarding && !d.AN.bound() {
		return fmt.Errorf("the downlink cannot forward with no access-network endpoint")
	}

	return nil
}

func (d dataPlane) rules() (pdrs []models.PDR, fars []models.FAR, qers []models.QER, urrs []models.URR) {
	ohr := models.OuterHeaderRemovalGtpUUdpIP

	pdrs = []models.PDR{{
		PDRID:              pdrIDUplink,
		Precedence:         pdrPrecedenceDefault,
		OuterHeaderRemoval: &ohr,
		FARID:              farIDUplink,
		QERID:              qerIDDefault,
		URRID:              urrIDUplink,
		PDI: models.PDI{
			SourceInterface: models.InterfaceAccess,
			LocalFTEID:      &models.FTEID{ChooseID: chooseIDSession},
			QFI:             d.uplinkQFI(),
		},
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
			FARID:       farIDUplink,
			ApplyAction: models.ApplyAction{Forw: true},
			ForwardingParameters: &models.ForwardingParameters{
				DestinationInterface: models.InterfaceCore,
			},
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

	for _, b := range d.Bearers {
		bp, bf, bq := d.bearerRules(b)
		pdrs = append(pdrs, bp...)
		fars = append(fars, bf...)
		qers = append(qers, bq...)
	}

	if d.Forwarding != nil {
		pdrs = append(pdrs, models.PDR{
			PDRID:              pdrIDForwarding,
			Precedence:         pdrPrecedenceDefault,
			OuterHeaderRemoval: &ohr,
			FARID:              farIDForwarding,
			PDI: models.PDI{
				SourceInterface: models.InterfaceAccess,
				LocalFTEID:      &models.FTEID{ChooseID: chooseIDForwarding},
			},
		})

		fars = append(fars, models.FAR{
			FARID:                farIDForwarding,
			ApplyAction:          models.ApplyAction{Forw: true},
			ForwardingParameters: forwardingTunnelParameters(*d.Forwarding, d.Access),
		})
	}

	return pdrs, fars, qers, urrs
}

func forwardingTunnelParameters(an AnchorBinding, access AccessType) *models.ForwardingParameters {
	s1u := access == Access4G

	switch {
	case an.IPv6 != nil:
		return &models.ForwardingParameters{
			DestinationInterface: models.InterfaceAccess,
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv6,
				TEID:        an.TEID,
				IPv6Address: an.IPv6,
				S1U:         s1u,
			},
		}
	case an.IPv4 != nil:
		return &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        an.TEID,
				IPv4Address: an.IPv4.To4(),
				S1U:         s1u,
			},
		}
	default:
		return &models.ForwardingParameters{}
	}
}

func downlinkPDR(pdrID uint16, ueIP netip.Addr) models.PDR {
	return models.PDR{
		PDRID:      pdrID,
		Precedence: pdrPrecedenceDefault,
		FARID:      farIDDownlink,
		QERID:      qerIDDefault,
		URRID:      urrIDDownlink,
		PDI: models.PDI{
			SourceInterface: models.InterfaceCore,
			UEIPAddress:     ueIP,
		},
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
	return accessForwardingParameters(d.AN, d.Access)
}

func accessForwardingParameters(an AnchorBinding, access AccessType) *models.ForwardingParameters {
	params := &models.ForwardingParameters{DestinationInterface: models.InterfaceAccess}

	s1u := access == Access4G

	switch {
	case an.IPv6 != nil:
		params.OuterHeaderCreation = &models.OuterHeaderCreation{
			Description: models.OuterHeaderCreationGtpUUdpIpv6,
			TEID:        an.TEID,
			IPv6Address: an.IPv6,
			S1U:         s1u,
		}
	case an.IPv4 != nil:
		params.OuterHeaderCreation = &models.OuterHeaderCreation{
			Description: models.OuterHeaderCreationGtpUUdpIpv4,
			TEID:        an.TEID,
			IPv4Address: an.IPv4.To4(),
			S1U:         s1u,
		}
	}

	return params
}

func (d dataPlane) establishRequest(seid uint64, imsi, policyID string, framedRoutes []netip.Prefix, localSwitch bool) *models.EstablishRequest {
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
		LocalSwitch:  localSwitch,
	}
}

func (d dataPlane) modifyRequest(prev dataPlane, seid uint64, policyID string) *models.ModifyRequest {
	pdrs, fars, qers, _ := d.rules()
	prevPDRs, prevFARs, prevQERs, _ := prev.rules()

	return &models.ModifyRequest{
		SEID:       seid,
		PolicyID:   policyID,
		UpdatePDRs: pdrs,
		UpdateFARs: fars,
		UpdateQERs: qers,
		RemovePDRs: removedIDs(prevPDRs, pdrs, func(p models.PDR) uint16 { return p.PDRID }),
		RemoveFARs: removedIDs(prevFARs, fars, func(f models.FAR) uint32 { return f.FARID }),
		RemoveQERs: removedIDs(prevQERs, qers, func(q models.QER) uint32 { return q.QERID }),
	}
}

func removedIDs[T any, ID comparable](prev, next []T, id func(T) ID) []ID {
	var out []ID

	for _, p := range prev {
		if !slices.ContainsFunc(next, func(n T) bool { return id(n) == id(p) }) {
			out = append(out, id(p))
		}
	}

	return out
}
