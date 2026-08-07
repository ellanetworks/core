// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// heldSession is what the UPF holds for one session, the left-hand side of a
// reconciliation.
type heldSession struct {
	pdrs         map[uint32]SPDRInfo
	urrs         map[uint32]struct{}
	framedRoutes []netip.Prefix
	ueIPv4       netip.Addr
	ueIPv6       netip.Addr
}

// plannedPDR is one PDR of the desired state resolved against what is held.
type plannedPDR struct {
	desired SPDRInfo

	// previous is the entry currently installed for this PDR ID, valid when held.
	previous SPDRInfo
	held     bool

	// supersedes marks that previous is installed under a map key desired does
	// not use, so that entry has to be removed once desired is written.
	supersedes bool

	// changed is false when the installed entry already matches desired, which
	// makes writing it again a no-op.
	changed bool
}

// sessionPlan converges what the UPF holds to a stated SessionState. Every rule
// is decided here: absent from the desired state means removed, present means
// exactly as stated.
type sessionPlan struct {
	fars map[uint32]ebpf.FarInfo
	qers map[uint32]ebpf.QerInfo

	ueIPv4 netip.Addr
	ueIPv6 netip.Addr

	pdrs       []plannedPDR
	removePDRs []SPDRInfo

	createURRs []uint32
	removeURRs []uint32

	addRoutes    []netip.Prefix
	removeRoutes []netip.Prefix

	policyID string
}

// localFTEIDs is the pool of local F-TEIDs the UPF owns. Uplink PDRs keep the
// one they hold; only a PDR without one draws from it.
type localFTEIDs struct {
	allocate func() (uint32, error)
	release  func(uint32)
}

// filterSource resolves the SDF filter slot a policy's PDRs point at.
type filterSource func(policyID string, direction models.Direction) uint32

// planSession resolves a stated SessionState against what the UPF holds.
//
// Every PDR starts from the entry held under its ID, so the UPF-owned local
// F-TEID survives; everything else comes from the statement. The local IPv4 and
// IPv6 are the N3 transport addresses the forwarding rules encapsulate from.
func planSession(
	held heldSession,
	desired *models.SessionState,
	localIPv4, localIPv6 netip.Addr,
	fteids localFTEIDs,
	filterIndex filterSource,
) (*sessionPlan, error) {
	plan := &sessionPlan{
		fars:     make(map[uint32]ebpf.FarInfo, len(desired.FARs)),
		qers:     make(map[uint32]ebpf.QerInfo, len(desired.QERs)),
		policyID: desired.PolicyID,
	}

	for _, far := range desired.FARs {
		plan.fars[far.FARID] = farInfoFromModel(far, localIPv4, localIPv6)
	}

	for _, qer := range desired.QERs {
		plan.qers[qer.QERID] = qerInfoFromModel(qer)
	}

	// The uplink source check authorises whichever families the downlink PDRs
	// name, so both are known before any uplink PDR is resolved.
	for _, pdr := range desired.PDRs {
		if pdr.PDI.LocalFTEID != nil || !pdr.PDI.UEIPAddress.IsValid() {
			continue
		}

		if pdr.PDI.UEIPAddress.Is4() {
			plan.ueIPv4 = pdr.PDI.UEIPAddress
		} else {
			plan.ueIPv6 = pdr.PDI.UEIPAddress
		}
	}

	// applyPDR stamps the authorised source addresses into the uplink map entry,
	// so a change to them restates every uplink PDR whatever else is unchanged.
	uplinkSourceChanged := plan.ueIPv4 != held.ueIPv4 || plan.ueIPv6 != held.ueIPv6

	// A statement is resolved whole or not at all, so an F-TEID drawn for one PDR
	// goes back to the pool when a later one cannot be resolved.
	var drawn []uint32

	defer func() {
		for _, teid := range drawn {
			fteids.release(teid)
		}
	}()

	stated := make(map[uint32]struct{}, len(desired.PDRs))

	for _, pdr := range desired.PDRs {
		id := uint32(pdr.PDRID)
		stated[id] = struct{}{}

		previous, isHeld := held.pdrs[id]

		spdrInfo := SPDRInfo{
			PdrID: id,
			PdrInfo: ebpf.PdrInfo{
				SEID:  desired.SEID,
				PdrID: id,
				IMSI:  desired.IMSI,
			},
		}

		// The local F-TEID is the UPF's to hold for the session's lifetime: a
		// re-allocation strands the RAN on a tunnel endpoint nothing reads.
		if isHeld {
			spdrInfo.TeID = previous.TeID
		}

		if err := extractPDR(pdr, &spdrInfo, plan.fars, plan.qers, fteids.allocate); err != nil {
			return nil, fmt.Errorf("PDR %d: %w", pdr.PDRID, err)
		}

		if spdrInfo.Allocated {
			drawn = append(drawn, spdrInfo.TeID)
		}

		spdrInfo.PdrInfo.FilterMapIndex = ebpf.NoFilterIndex

		if desired.PolicyID != "" {
			direction := models.DirectionUplink
			if spdrInfo.UEIP.IsValid() {
				direction = models.DirectionDownlink
			}

			spdrInfo.PdrInfo.FilterMapIndex = filterIndex(desired.PolicyID, direction)
		}

		planned := plannedPDR{desired: spdrInfo, previous: previous, held: isHeld}

		switch {
		case !isHeld:
			planned.changed = true
		case previous.PdrInfo != spdrInfo.PdrInfo || previous.UEIP != spdrInfo.UEIP || previous.TeID != spdrInfo.TeID:
			planned.changed = true
		case uplinkSourceChanged && !spdrInfo.UEIP.IsValid():
			planned.changed = true
		}

		planned.supersedes = isHeld && pdrKeyChanged(previous, spdrInfo)

		plan.pdrs = append(plan.pdrs, planned)
	}

	for id, heldPDR := range held.pdrs {
		if _, ok := stated[id]; !ok {
			plan.removePDRs = append(plan.removePDRs, heldPDR)
		}
	}

	// NewUrr zeroes the counter, so only a URR the session does not already hold
	// is created: recreating one loses the bytes accounted since the last poll.
	statedURRs := make(map[uint32]struct{}, len(desired.URRs))

	for _, urr := range desired.URRs {
		if _, dup := statedURRs[urr.URRID]; dup {
			continue
		}

		statedURRs[urr.URRID] = struct{}{}

		if _, ok := held.urrs[urr.URRID]; !ok {
			plan.createURRs = append(plan.createURRs, urr.URRID)
		}
	}

	for id := range held.urrs {
		if _, ok := statedURRs[id]; !ok {
			plan.removeURRs = append(plan.removeURRs, id)
		}
	}

	plan.addRoutes, plan.removeRoutes = planFramedRoutes(held.framedRoutes, desired.FramedRoutes, plan.ueIPv4, plan.ueIPv6)

	drawn = nil

	return plan, nil
}

// planFramedRoutes converges the session's framed-route prefixes (TS 23.501
// §5.6.14). A route with no downlink PDR of its own family cannot redirect
// anywhere, so it is left out: a dormant route must not deny the UE the rest of
// its connectivity.
func planFramedRoutes(held, desired []netip.Prefix, ueIPv4, ueIPv6 netip.Addr) (add, remove []netip.Prefix) {
	installable := make(map[netip.Prefix]struct{}, len(desired))

	for _, fr := range desired {
		if fr.Addr().Is4() {
			if !ueIPv4.IsValid() {
				continue
			}
		} else if !ueIPv6.IsValid() {
			continue
		}

		installable[fr] = struct{}{}
	}

	heldSet := make(map[netip.Prefix]struct{}, len(held))
	for _, fr := range held {
		heldSet[fr] = struct{}{}
	}

	for _, fr := range desired {
		if _, ok := installable[fr]; !ok {
			continue
		}

		if _, ok := heldSet[fr]; !ok {
			add = append(add, fr)
		}
	}

	for _, fr := range held {
		if _, ok := installable[fr]; !ok {
			remove = append(remove, fr)
		}
	}

	return add, remove
}

// ueAddressFor returns the UE address a framed route of the prefix's family
// redirects to.
func ueAddressFor(fr netip.Prefix, ueIPv4, ueIPv6 netip.Addr) netip.Addr {
	if fr.Addr().Is4() {
		return ueIPv4
	}

	return ueIPv6
}
