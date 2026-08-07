// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

var (
	planLocalIPv4 = netip.MustParseAddr("10.3.0.1")
	planLocalIPv6 = netip.MustParseAddr("2001:db8::1")

	planUEIPv4   = netip.MustParseAddr("10.45.0.1")
	planUEIPv6   = netip.MustParseAddr("2001:db8:1::")
	planGnbIPv4  = "192.168.0.10"
	planRouteV4  = netip.MustParsePrefix("192.168.50.0/24")
	planRouteV6  = netip.MustParsePrefix("2001:db8:9::/64")
	planNoFilter = func(string, models.Direction) uint32 { return ebpf.NoFilterIndex }
)

// countingTEIDs hands out 1, 2, 3… and records what is outstanding.
type countingTEIDs struct {
	next     uint32
	issued   int
	released int
}

func (c *countingTEIDs) pool() localFTEIDs {
	return localFTEIDs{
		allocate: func() (uint32, error) {
			c.next++
			c.issued++

			return c.next, nil
		},
		release: func(uint32) { c.released++ },
	}
}

// outstanding is how many F-TEIDs the pool has handed out and not taken back.
func (c *countingTEIDs) outstanding() int { return c.issued - c.released }

func failingTEIDs() localFTEIDs {
	return localFTEIDs{
		allocate: func() (uint32, error) { return 0, fmt.Errorf("no resources available") },
		release:  func(uint32) {},
	}
}

// boundDownlinkFAR forwards to a gNB N3 endpoint over IPv4 GTP-U.
func boundDownlinkFAR() models.FAR {
	return models.FAR{
		FARID:       2,
		ApplyAction: models.ApplyAction{Forw: true},
		ForwardingParameters: &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        0x6000,
				IPv4Address: net.ParseIP(planGnbIPv4).To4(),
			},
		},
	}
}

// dualStackState is the state the SMF states for a dual-stack session: an
// uplink PDR over GTP-U, a downlink PDR per family, one forwarding rule shared
// by the downlinks, one QoS rule and two usage rules.
const planSEID = uint64(1)

func dualStackState() *models.SessionState {
	return &models.SessionState{
		SEID: planSEID,
		IMSI: "001010000000001",
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 2, QERID: 1, URRID: 2, PDI: models.PDI{UEIPAddress: planUEIPv4}},
			{PDRID: 3, FARID: 2, QERID: 1, URRID: 2, PDI: models.PDI{UEIPAddress: planUEIPv6}},
		},
		FARs: []models.FAR{
			{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}, ForwardingParameters: &models.ForwardingParameters{}},
			boundDownlinkFAR(),
		},
		QERs: []models.QER{{QERID: 1, QFI: 9, MBR: &models.MBR{ULMBR: 100, DLMBR: 200}}},
		URRs: []models.URR{{URRID: 1}, {URRID: 2}},
	}
}

func planFor(t *testing.T, held heldSession, desired *models.SessionState, fteids localFTEIDs) *sessionPlan {
	t.Helper()

	plan, err := planSession(held, desired, planLocalIPv4, planLocalIPv6, fteids, planNoFilter)
	if err != nil {
		t.Fatalf("plan session: %v", err)
	}

	return plan
}

// commit is what applying a plan leaves the engine holding, so a test can state
// the same session twice without the datapath.
func commit(held heldSession, plan *sessionPlan) heldSession {
	next := heldSession{
		pdrs:   make(map[uint32]SPDRInfo, len(plan.pdrs)),
		urrs:   make(map[uint32]struct{}, len(held.urrs)),
		ueIPv4: plan.ueIPv4,
		ueIPv6: plan.ueIPv6,
	}

	for _, planned := range plan.pdrs {
		next.pdrs[planned.desired.PdrID] = planned.desired
	}

	for id := range held.urrs {
		next.urrs[id] = struct{}{}
	}

	for _, id := range plan.createURRs {
		next.urrs[id] = struct{}{}
	}

	for _, id := range plan.removeURRs {
		delete(next.urrs, id)
	}

	routes := make(map[netip.Prefix]struct{}, len(held.framedRoutes))
	for _, fr := range held.framedRoutes {
		routes[fr] = struct{}{}
	}

	for _, fr := range plan.removeRoutes {
		delete(routes, fr)
	}

	for _, fr := range plan.addRoutes {
		routes[fr] = struct{}{}
	}

	for fr := range routes {
		next.framedRoutes = append(next.framedRoutes, fr)
	}

	return next
}

func emptyHeld() heldSession {
	return heldSession{pdrs: map[uint32]SPDRInfo{}, urrs: map[uint32]struct{}{}}
}

func plannedByID(plan *sessionPlan, id uint32) plannedPDR {
	for _, planned := range plan.pdrs {
		if planned.desired.PdrID == id {
			return planned
		}
	}

	panic(fmt.Sprintf("plan names no PDR %d", id))
}

// A first statement creates everything it names.
func TestPlanSessionCreatesEverythingStated(t *testing.T) {
	teids := &countingTEIDs{}
	plan := planFor(t, emptyHeld(), dualStackState(), teids.pool())

	if len(plan.pdrs) != 3 {
		t.Fatalf("planned PDRs = %d, want 3", len(plan.pdrs))
	}

	for _, planned := range plan.pdrs {
		if !planned.changed {
			t.Errorf("PDR %d is not written on a first statement", planned.desired.PdrID)
		}

		if planned.held || planned.supersedes {
			t.Errorf("PDR %d claims prior state on a first statement: %+v", planned.desired.PdrID, planned)
		}
	}

	if got := plannedByID(plan, 1).desired.TeID; got != 1 {
		t.Errorf("uplink local F-TEID = %d, want the allocated 1", got)
	}

	if teids.outstanding() != 1 {
		t.Errorf("F-TEIDs outstanding = %d, want 1: only the uplink PDR names a local tunnel", teids.outstanding())
	}

	if len(plan.createURRs) != 2 || len(plan.removeURRs) != 0 {
		t.Errorf("URRs created/removed = %v/%v, want both stated URRs created", plan.createURRs, plan.removeURRs)
	}

	if plan.ueIPv4 != planUEIPv4 || plan.ueIPv6 != planUEIPv6 {
		t.Errorf("authorised uplink sources = %v/%v, want %v/%v", plan.ueIPv4, plan.ueIPv6, planUEIPv4, planUEIPv6)
	}
}

// Restating an unchanged session writes nothing: the datapath already holds it.
func TestPlanSessionRestatementIsANoOp(t *testing.T) {
	teids := &countingTEIDs{}

	first := planFor(t, emptyHeld(), dualStackState(), teids.pool())
	held := commit(emptyHeld(), first)

	second := planFor(t, held, dualStackState(), teids.pool())

	for _, planned := range second.pdrs {
		if planned.changed {
			t.Errorf("PDR %d is rewritten by an identical statement: %+v", planned.desired.PdrID, planned.desired.PdrInfo)
		}
	}

	if len(second.createURRs) != 0 || len(second.removeURRs) != 0 {
		t.Errorf("URRs created/removed on a restatement = %v/%v, want none: creating one zeroes its counter",
			second.createURRs, second.removeURRs)
	}

	if len(second.removePDRs) != 0 {
		t.Errorf("PDRs removed by an identical statement = %+v, want none", second.removePDRs)
	}

	if teids.outstanding() != 1 {
		t.Errorf("F-TEIDs outstanding across two statements = %d, want 1: the UPF holds the one it issued", teids.outstanding())
	}

	if got := plannedByID(second, 1).desired.TeID; got != 1 {
		t.Errorf("uplink local F-TEID after a restatement = %d, want it held at 1", got)
	}
}

// A rule the statement stops naming is withdrawn. Absence is removal, never
// "unchanged".
func TestPlanSessionWithdrawsUnstatedRules(t *testing.T) {
	teids := &countingTEIDs{}

	held := commit(emptyHeld(), planFor(t, emptyHeld(), dualStackState(), teids.pool()))

	desired := dualStackState()
	desired.PDRs = desired.PDRs[:2] // drop the IPv6 downlink
	desired.URRs = desired.URRs[:1] // drop URR 2

	plan := planFor(t, held, desired, teids.pool())

	if len(plan.removePDRs) != 1 || plan.removePDRs[0].PdrID != 3 {
		t.Fatalf("PDRs withdrawn = %+v, want PDR 3", plan.removePDRs)
	}

	if len(plan.removeURRs) != 1 || plan.removeURRs[0] != 2 {
		t.Fatalf("URRs withdrawn = %v, want [2]", plan.removeURRs)
	}
}

// The forwarding rule is restated in full: withdrawing the outer header
// creation leaves no tunnel endpoint behind, and the PDRs that name the rule are
// rewritten with it.
func TestPlanSessionWithdrawnEndpointReachesThePDRs(t *testing.T) {
	teids := &countingTEIDs{}

	held := commit(emptyHeld(), planFor(t, emptyHeld(), dualStackState(), teids.pool()))

	suspended := dualStackState()
	suspended.FARs[1] = models.FAR{
		FARID:                2,
		ApplyAction:          models.ApplyAction{Buff: true, Nocp: true},
		ForwardingParameters: &models.ForwardingParameters{},
	}

	plan := planFor(t, held, suspended, teids.pool())

	far := plan.fars[2]

	var zeroIP [16]byte

	if far.TeID != 0 || far.OuterHeaderCreation != 0 || far.RemoteIP != zeroIP || far.LocalIP != zeroIP {
		t.Errorf("suspended forwarding rule still names an endpoint: %+v", far)
	}

	for _, id := range []uint32{2, 3} {
		planned := plannedByID(plan, id)
		if !planned.changed {
			t.Errorf("downlink PDR %d is not rewritten after its forwarding rule changed", id)
		}

		if planned.desired.PdrInfo.Far != far {
			t.Errorf("downlink PDR %d carries forwarding rule %+v, want %+v", id, planned.desired.PdrInfo.Far, far)
		}
	}

	if plannedByID(plan, 1).changed {
		t.Error("uplink PDR is rewritten by a change to a forwarding rule it does not name")
	}
}

// A QoS change reaches every PDR that names the rule.
func TestPlanSessionQoSChangeReachesThePDRs(t *testing.T) {
	teids := &countingTEIDs{}

	held := commit(emptyHeld(), planFor(t, emptyHeld(), dualStackState(), teids.pool()))

	reQoS := dualStackState()
	reQoS.QERs[0].MBR = &models.MBR{ULMBR: 300, DLMBR: 400}

	plan := planFor(t, held, reQoS, teids.pool())

	if got := plan.qers[1].MaxBitrateUL; got != 300*1000 {
		t.Errorf("uplink MBR = %d, want %d", got, 300*1000)
	}

	for _, planned := range plan.pdrs {
		if !planned.changed {
			t.Errorf("PDR %d is not rewritten after the QoS rule it names changed", planned.desired.PdrID)
		}
	}
}

// A downlink PDR moved to a different UE address supersedes its previous map
// entry, which has to go: it holds a downlink slot and keeps authorising the
// old address.
func TestPlanSessionUEAddressChangeSupersedesTheOldEntry(t *testing.T) {
	teids := &countingTEIDs{}

	held := commit(emptyHeld(), planFor(t, emptyHeld(), dualStackState(), teids.pool()))

	moved := dualStackState()
	newV4 := netip.MustParseAddr("10.45.0.2")
	moved.PDRs[1].PDI.UEIPAddress = newV4

	plan := planFor(t, held, moved, teids.pool())

	downlink := plannedByID(plan, 2)
	if !downlink.supersedes {
		t.Error("the downlink PDR's previous entry is not superseded after its UE address changed")
	}

	if downlink.previous.UEIP != planUEIPv4 {
		t.Errorf("superseded entry = %v, want the previous UE address %v", downlink.previous.UEIP, planUEIPv4)
	}

	// The uplink source check is stamped into the uplink map entry, so it is
	// rewritten even though the uplink PDR itself is unchanged.
	if !plannedByID(plan, 1).changed {
		t.Error("the uplink PDR is not rewritten after the authorised source address changed")
	}

	if plan.ueIPv4 != newV4 {
		t.Errorf("authorised uplink IPv4 = %v, want %v", plan.ueIPv4, newV4)
	}
}

// A framed route whose family has no downlink PDR cannot redirect anywhere, so
// it is left out. A dormant route must not deny the UE the rest of its
// connectivity.
func TestPlanSessionSkipsFramedRouteWithNoDownlinkOfItsFamily(t *testing.T) {
	teids := &countingTEIDs{}

	desired := dualStackState()
	desired.PDRs = desired.PDRs[:2] // IPv4 only
	desired.FramedRoutes = []netip.Prefix{planRouteV4, planRouteV6}

	plan := planFor(t, emptyHeld(), desired, teids.pool())

	if len(plan.addRoutes) != 1 || plan.addRoutes[0] != planRouteV4 {
		t.Fatalf("framed routes installed = %v, want [%s]", plan.addRoutes, planRouteV4)
	}

	if got := ueAddressFor(planRouteV4, plan.ueIPv4, plan.ueIPv6); got != planUEIPv4 {
		t.Errorf("framed route redirects to %v, want the UE's IPv4 %v", got, planUEIPv4)
	}
}

// Framed routes converge like every other rule: one the statement stops naming
// is withdrawn, and one it keeps is not rewritten.
func TestPlanSessionConvergesFramedRoutes(t *testing.T) {
	teids := &countingTEIDs{}

	first := dualStackState()
	first.FramedRoutes = []netip.Prefix{planRouteV4, planRouteV6}

	held := commit(emptyHeld(), planFor(t, emptyHeld(), first, teids.pool()))

	second := dualStackState()
	second.FramedRoutes = []netip.Prefix{planRouteV6}

	plan := planFor(t, held, second, teids.pool())

	if len(plan.addRoutes) != 0 {
		t.Errorf("framed routes installed again = %v, want none", plan.addRoutes)
	}

	if len(plan.removeRoutes) != 1 || plan.removeRoutes[0] != planRouteV4 {
		t.Errorf("framed routes withdrawn = %v, want [%s]", plan.removeRoutes, planRouteV4)
	}
}

// The policy binding is stated, so it converges rather than being left alone
// when unstated.
func TestPlanSessionCarriesThePolicyBinding(t *testing.T) {
	teids := &countingTEIDs{}

	desired := dualStackState()
	desired.PolicyID = "policy-a"

	filters := func(policyID string, direction models.Direction) uint32 {
		if policyID != "policy-a" {
			t.Fatalf("filter slot resolved for policy %q, want policy-a", policyID)
		}

		if direction == models.DirectionUplink {
			return 7
		}

		return 8
	}

	plan, err := planSession(emptyHeld(), desired, planLocalIPv4, planLocalIPv6, teids.pool(), filters)
	if err != nil {
		t.Fatalf("plan session: %v", err)
	}

	if got := plannedByID(plan, 1).desired.PdrInfo.FilterMapIndex; got != 7 {
		t.Errorf("uplink PDR filter slot = %d, want 7", got)
	}

	for _, id := range []uint32{2, 3} {
		if got := plannedByID(plan, id).desired.PdrInfo.FilterMapIndex; got != 8 {
			t.Errorf("downlink PDR %d filter slot = %d, want 8", id, got)
		}
	}
}

// A PDR that names neither a local F-TEID nor a UE address matches nothing, so
// the whole statement is refused before anything reaches the datapath.
func TestPlanSessionRefusesAPDRThatMatchesNothing(t *testing.T) {
	desired := &models.SessionState{
		SEID: 1,
		PDRs: []models.PDR{{PDRID: 1, FARID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Drop: true}}},
	}

	if _, err := planSession(emptyHeld(), desired, planLocalIPv4, planLocalIPv6, failingTEIDs(), planNoFilter); err == nil {
		t.Fatal("expected a PDR naming neither a local F-TEID nor a UE address to be refused")
	}
}

// A statement resolved part-way and then refused gives back every F-TEID it
// drew: the session it was for is never created, so nothing else reclaims them.
func TestPlanSessionReturnsFTEIDsWhenTheStatementIsRefused(t *testing.T) {
	teids := &countingTEIDs{}

	desired := dualStackState()
	desired.PDRs = append(desired.PDRs, models.PDR{PDRID: 4, FARID: 1, PDI: models.PDI{}})

	if _, err := planSession(emptyHeld(), desired, planLocalIPv4, planLocalIPv6, teids.pool(), planNoFilter); err == nil {
		t.Fatal("expected a PDR naming neither a local F-TEID nor a UE address to be refused")
	}

	if teids.issued == 0 {
		t.Fatal("the refused statement drew no F-TEID, so it proves nothing")
	}

	if teids.outstanding() != 0 {
		t.Errorf("F-TEIDs outstanding after a refused statement = %d, want 0", teids.outstanding())
	}
}

// An exhausted F-TEID pool refuses the statement.
func TestPlanSessionRefusesWhenNoFTEIDIsAvailable(t *testing.T) {
	if _, err := planSession(emptyHeld(), dualStackState(), planLocalIPv4, planLocalIPv6, failingTEIDs(), planNoFilter); err == nil {
		t.Fatal("expected an exhausted F-TEID pool to refuse the statement")
	}
}

// A URR the session already holds is never created again: creating one zeroes
// its counter, dropping every byte accounted since the last poll.
func TestPlanSessionCreatesOnlyURRsTheSessionLacks(t *testing.T) {
	held := emptyHeld()
	held.urrs[1] = struct{}{}

	plan := planFor(t, held, dualStackState(), (&countingTEIDs{}).pool())

	if len(plan.createURRs) != 1 || plan.createURRs[0] != 2 {
		t.Fatalf("URRs created = %v, want [2]: URR 1 is already held and its counter must survive", plan.createURRs)
	}
}

// The IMSI travels with every PDR, so a usage or flow record can name the
// subscriber whichever statement wrote the rule.
func TestPlanSessionStampsTheIMSIOnEveryPDR(t *testing.T) {
	plan := planFor(t, emptyHeld(), dualStackState(), (&countingTEIDs{}).pool())

	for _, planned := range plan.pdrs {
		if planned.desired.PdrInfo.IMSI != "001010000000001" {
			t.Errorf("PDR %d IMSI = %q, want 001010000000001", planned.desired.PdrID, planned.desired.PdrInfo.IMSI)
		}
	}
}

// Outer header removal is stated in full: a statement that drops it leaves none
// behind.
func TestPlanSessionWithdrawnOuterHeaderRemoval(t *testing.T) {
	teids := &countingTEIDs{}

	ohr := models.OuterHeaderRemovalGtpUUdpIpv6

	withRemoval := dualStackState()
	withRemoval.PDRs[0].OuterHeaderRemoval = &ohr

	held := commit(emptyHeld(), planFor(t, emptyHeld(), withRemoval, teids.pool()))

	if got := held.pdrs[1].PdrInfo.OuterHeaderRemoval; got != models.OuterHeaderRemovalGtpUUdpIpv6 {
		t.Fatalf("outer header removal = %d, want %d", got, models.OuterHeaderRemovalGtpUUdpIpv6)
	}

	plan := planFor(t, held, dualStackState(), teids.pool())

	uplink := plannedByID(plan, 1)
	if uplink.desired.PdrInfo.OuterHeaderRemoval != 0 {
		t.Errorf("outer header removal = %d, want 0 once the statement stops naming one", uplink.desired.PdrInfo.OuterHeaderRemoval)
	}

	if !uplink.changed {
		t.Error("the uplink PDR is not rewritten after its outer header removal was withdrawn")
	}
}
