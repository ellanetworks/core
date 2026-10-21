// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

const defaultRuleUplinkPDR = 256 + 4*60

func defaultQoSRule() smf.PCCRule {
	r := floorControlRule()
	r.QCI = 9
	r.ARP = models.Arp{PriorityLevel: 1, PreemptCap: models.PreemptionCapabilityNotPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable}

	return r
}

func defaultRulePDR(m *models.ModifyRequest, id uint16) (models.PDR, bool) {
	i := slices.IndexFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.PDRID == id })
	if i < 0 {
		return models.PDR{}, false
	}

	return m.UpdatePDRs[i], true
}

func removedSince(upf *fakeUPF, n int, pdr uint16) bool {
	upf.mu.Lock()
	defer upf.mu.Unlock()

	return slices.ContainsFunc(upf.modifyCalls[n:], func(m *models.ModifyRequest) bool { return slices.Contains(m.RemovePDRs, pdr) })
}

func TestRuleWithTheDefaultQoSBindsToTheDefaultBearer_TS23203_6_1_1_4(t *testing.T) {
	s, pcf, upf, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, defaultQoSRule())

	if n := len(mmeCb.dedicatedActivations()); n != 0 {
		t.Fatalf("%d dedicated bearer activations, want the rule bound to the default bearer", n)
	}

	m := lastModify(t, upf)

	uplink, ok := defaultRulePDR(m, defaultRuleUplinkPDR)
	if !ok || uplink.PDI.LocalFTEID == nil || uplink.PDI.LocalFTEID.ChooseID != 1 || uplink.FARID != 1 || len(uplink.PDI.SDFFilters) != 1 || uplink.Precedence >= 256 {
		t.Fatalf("UPF PDRs %+v, want the rule's uplink on the default bearer's F-TEID ahead of its catch-all", m.UpdatePDRs)
	}

	downlink, ok := defaultRulePDR(m, defaultRuleUplinkPDR+1)
	if !ok || downlink.FARID != 2 || !downlink.PDI.UEIPAddress.IsValid() || downlink.QERID != uplink.QERID {
		t.Fatalf("UPF PDRs %+v, want the rule's downlink through the default bearer's FAR", m.UpdatePDRs)
	}

	if !slices.ContainsFunc(m.UpdateQERs, func(q models.QER) bool { return q.QERID == uplink.QERID && q.MBR != nil && q.MBR.DLMBR == 8 }) {
		t.Fatalf("UPF QERs %+v, want the rule's 8 kbps MBR", m.UpdateQERs)
	}

	closed := defaultQoSRule()
	closed.Version = 1
	closed.Gate = models.GateStatus{ULGate: models.GateClose, DLGate: models.GateClose}
	pushRules(t, s, ref, closed)

	if !slices.ContainsFunc(lastModify(t, upf).UpdateQERs, func(q models.QER) bool {
		return q.QERID == uplink.QERID && q.GateStatus != nil && q.GateStatus.ULGate == models.GateClose && q.GateStatus.DLGate == models.GateClose
	}) {
		t.Fatal("the disabled rule's gates are open on the default bearer (TS 29.212 §4.5.2.3)")
	}

	sent := modifyCount(upf)

	moved := closed
	moved.Version = 2
	moved.ARP = floorControlARP
	pushRules(t, s, ref, moved)

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d dedicated bearer activations, want one once the rule's ARP differs from the default bearer's", n)
	}

	if !removedSince(upf, sent, defaultRuleUplinkPDR) {
		t.Fatal("the rule's PDRs stayed on the default bearer after it moved to a dedicated bearer")
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none", r)
	}
}

func TestRuleWithTheDefaultQoSBindsToTheDefaultQoSFlow_TS23203_6_1_1_4(t *testing.T) {
	s, pcf, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, defaultQoSRule())

	time.Sleep(100 * time.Millisecond)

	if n := len(amfCb.modifications()); n != 0 {
		t.Fatalf("%d QoS flow modifications, want the rule on the default QoS flow without signalling", n)
	}

	m := lastModify(t, upf)

	uplink, ok := defaultRulePDR(m, defaultRuleUplinkPDR)
	if !ok || uplink.PDI.QFI != 1 || uplink.PDI.LocalFTEID == nil || uplink.PDI.LocalFTEID.ChooseID != 1 {
		t.Fatalf("UPF PDRs %+v, want the rule's uplink on the default QoS flow", m.UpdatePDRs)
	}

	pushRules(t, s, ref)

	if m := lastModify(t, upf); !slices.Contains(m.RemovePDRs, defaultRuleUplinkPDR) {
		t.Fatalf("UPF modification removes PDRs %v, want the rule's PDRs gone", m.RemovePDRs)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none", r)
	}
}

func TestSessionViewListsBearersAndTheirRules(t *testing.T) {
	s, _, _, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, defaultQoSRule(), withID(floorControlRule(), "af;mcptt#3.1"))

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d activations, want one for the floor control rule", n)
	}

	if err := s.DedicatedBearerActivated(context.Background(), ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	got := s.GetSession(ref).Bearers()
	if len(got) != 2 {
		t.Fatalf("bearers %+v, want the default and the floor control bearer", got)
	}

	if d := got[0]; !d.Default || d.ID != epsTestEBI || d.QCI != 9 || !slices.Equal(d.Rules, []string{"af;mcptt#2.1"}) {
		t.Fatalf("default bearer %+v, want EBI %d at QCI 9 carrying the rule with its QoS", d, epsTestEBI)
	}

	if b := got[1]; b.Default || b.ID != 6 || b.QCI != 69 || b.ARP != floorControlARP || !slices.Equal(b.Rules, []string{"af;mcptt#3.1"}) {
		t.Fatalf("dedicated bearer %+v, want EBI 6 at QCI 69 carrying the floor control rule", b)
	}
}

func withID(r smf.PCCRule, id string) smf.PCCRule {
	r.ID = id

	return r
}
