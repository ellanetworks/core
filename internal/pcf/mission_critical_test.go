// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf_test

import (
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

const mcpttICSI = "urn:urn-7:3gpp-service.ims.icsi.mcptt"

func mcpttAAR(identifier string, priority *rx.ReservationPriority) rx.AARequest {
	r := callAAR(50000, rx.ServiceInfoFinal)
	r.AFApplicationIdentifier = mcpttICSI
	r.MCPTTIdentifier = identifier
	r.ReservationPriority = priority

	application := rx.MediaApplication
	r.MediaComponents = append(r.MediaComponents, rx.MediaComponent{
		Number:                  2,
		Type:                    &application,
		MaxRequestedBandwidthUL: ptr(tgpp.Bandwidth(8000)),
		MaxRequestedBandwidthDL: ptr(tgpp.Bandwidth(8000)),
		SubComponents: []rx.MediaSubComponent{{
			FlowNumber: 1,
			FlowDescriptions: []string{
				"permit out 17 from 192.0.2.10 49002 to 10.60.0.1 50002",
				"permit in 17 from 10.60.0.1 50002 to 192.0.2.10 49002",
			},
		}},
	})

	return r
}

func ruleByID(t *testing.T, d *smf.PolicyDecision, id string) smf.PCCRule {
	t.Helper()

	i := slices.IndexFunc(d.Rules, func(r smf.PCCRule) bool { return r.ID == id })
	if i < 0 {
		t.Fatalf("no rule %s in %+v", id, d.Rules)
	}

	return d.Rules[i]
}

func TestMCPTTMediaQoS_TS23289_4_3_3_2(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;mcptt", mcpttAAR("mcpttp", ptr(rx.ReservationPriority(11)))), success)

	d := n.next(t)

	voice := ruleByID(t, d, "af;mcptt#1.1")
	if voice.QCI != 65 || voice.GBR.Uplink.Bps() != 41000 || voice.GBR.Downlink.Bps() != 41000 {
		t.Fatalf("voice rule %+v, want QCI 65 with a 41 kbps GBR", voice)
	}

	floor := ruleByID(t, d, "af;mcptt#2.1")
	if floor.QCI != 65 || floor.GBR.Uplink.Bps() != 8000 || floor.GBR.Downlink.Bps() != 8000 || floor.MBR.Uplink.Bps() != 8000 || floor.ARP != voice.ARP {
		t.Fatalf("floor control rule %+v, want QCI 65 at the voice ARP with an 8 kbps GBR, to share its bearer (TS 23.289 §4.3.3.2)", floor)
	}
}

func TestMCPTTARP_TS29213Table632Note5(t *testing.T) {
	arp := func(level int32, capability models.PreemptionCapability, vulnerability models.PreemptionVulnerability) models.Arp {
		return models.Arp{PriorityLevel: level, PreemptCap: capability, PreemptVuln: vulnerability}
	}

	may, never := models.PreemptionCapabilityMayPreempt, models.PreemptionCapabilityNotPreempt
	exposed, protected := models.PreemptionVulnerabilityPreemptable, models.PreemptionVulnerabilityNotPreemptable

	for _, tc := range []struct {
		name       string
		identifier string
		priority   *rx.ReservationPriority
		want       models.Arp
	}{
		{name: "emergency mcpttp.15", identifier: "mcpttp", priority: ptr(rx.ReservationPriority(15)), want: arp(1, may, protected)},
		{name: "imminent peril mcpttp.13", identifier: "mcpttp", priority: ptr(rx.ReservationPriority(13)), want: arp(2, may, protected)},
		{name: "normal mcpttp.11", identifier: "mcpttp", priority: ptr(rx.ReservationPriority(11)), want: arp(3, may, exposed)},
		{name: "lowest mcpttp.0", identifier: "mcpttp", priority: ptr(rx.ReservationPriority(0)), want: arp(8, may, exposed)},
		{name: "mcpttp without Reservation-Priority (DEFAULT)", identifier: "mcpttp", want: arp(8, may, exposed)},
		{name: "queueing mcpttq.15", identifier: "mcpttq", priority: ptr(rx.ReservationPriority(15)), want: arp(1, never, protected)},
		{name: "queueing mcpttq.11", identifier: "MCPTTQ", priority: ptr(rx.ReservationPriority(11)), want: arp(3, never, exposed)},
		{name: "provider name after the namespace", identifier: "mcpttp example.org", priority: ptr(rx.ReservationPriority(13)), want: arp(2, may, protected)},
		{name: "no MCPTT-Identifier", want: arp(8, never, exposed)},
		{name: "unknown namespace", identifier: "ets", priority: ptr(rx.ReservationPriority(15)), want: arp(8, never, exposed)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, n := newEnforcedPCF(t)

			requireResult(t, aa(t, p, "af;mcptt", mcpttAAR(tc.identifier, tc.priority)), success)

			d := n.next(t)
			for _, id := range []string{"af;mcptt#1.1", "af;mcptt#2.1"} {
				if got := ruleByID(t, d, id).ARP; got != tc.want {
					t.Fatalf("rule %s ARP %+v, want %+v", id, got, tc.want)
				}
			}
		})
	}
}

func TestMCPTTPriorityAdjustment_TS29214A13_1(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;mcptt", mcpttAAR("mcpttp", ptr(rx.ReservationPriority(11)))), success)

	normal := ruleByID(t, n.next(t), "af;mcptt#1.1")

	update := rx.RequestUpdate
	elevate := rx.AARequest{RequestType: &update, MCPTTIdentifier: "mcpttp", ReservationPriority: ptr(rx.ReservationPriority(15))}
	requireResult(t, aa(t, p, "af;mcptt", elevate), success)

	emergency := n.next(t)
	voice := ruleByID(t, emergency, "af;mcptt#1.1")

	if voice.ARP.PriorityLevel != 1 || voice.Version <= normal.Version || voice.QCI != 65 || !slices.Equal(voice.Filters, normal.Filters) {
		t.Fatalf("voice rule after elevation %+v, want the same rule at ARP 1 and a later version", voice)
	}

	if floor := ruleByID(t, emergency, "af;mcptt#2.1"); floor.ARP.PriorityLevel != 1 {
		t.Fatalf("floor control ARP %+v after elevation, want 1", floor.ARP)
	}

	other := rx.AARequest{RequestType: &update, SpecificActions: []rx.SpecificAction{rx.ActionIndicationOfFailedResourcesAllocation}}
	requireResult(t, aa(t, p, "af;mcptt", other), success)
	n.none(t)

	lower := rx.AARequest{RequestType: &update, MCPTTIdentifier: "mcpttp", ReservationPriority: ptr(rx.ReservationPriority(11))}
	requireResult(t, aa(t, p, "af;mcptt", lower), success)

	if voice := ruleByID(t, n.next(t), "af;mcptt#1.1"); voice.ARP.PriorityLevel != 3 {
		t.Fatalf("voice ARP %+v after returning to normal priority, want 3", voice.ARP)
	}
}

func TestVoLTEQoSIsUnchangedByMissionCriticalSupport(t *testing.T) {
	p, n := newEnforcedPCF(t)

	r := callAAR(50000, rx.ServiceInfoFinal)
	r.MCPTTIdentifier = "mcpttp"
	r.ReservationPriority = ptr(rx.ReservationPriority(15))

	requireResult(t, aa(t, p, "af;volte", r), success)

	voice := ruleByID(t, n.next(t), "af;volte#1.1")
	want := models.Arp{PriorityLevel: 2, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable}

	if voice.QCI != 1 || voice.ARP != want {
		t.Fatalf("voice rule %+v without the MCPTT ICSI, want QCI 1 at ARP %+v", voice, want)
	}
}
