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

func (f *fakePCF) reportedFailures() []reportedRule {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.reports)
}

func bearerUplinkPorts(t *testing.T, upf *fakeUPF) []uint16 {
	t.Helper()

	var ports []uint16

	for _, p := range bearerUplinkPDRs(lastModify(t, upf)) {
		for _, f := range p.PDI.SDFFilters {
			ports = append(ports, f.LocalPort)
		}
	}

	if len(ports) == 0 {
		t.Fatal("no bearer uplink PDR")
	}

	return ports
}

func requireModificationCount(t *testing.T, mmeCb *fakeMME, n int) {
	t.Helper()

	time.Sleep(100 * time.Millisecond)

	if got := len(mmeCb.dedicatedModifications()); got != n {
		t.Fatalf("%d dedicated bearer modifications sent, want %d", got, n)
	}
}

func movedVoiceRule() smf.PCCRule {
	r := voiceRule()
	r.Version = 1

	for i := range r.Filters {
		r.Filters[i].LocalPort = 50010
	}

	return r
}

func TestRejectedChangeKeepsTheEnforcedRule(t *testing.T) {
	s, pcf, upf, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	pushRules(t, s, ref, movedVoiceRule())
	waitDedicatedModifications(t, mmeCb, 1)

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, false)

	reports := pcf.reportedFailures()
	if len(reports) != 1 || reports[0].Rule.Version != 1 || reports[0].Active == nil || reports[0].Active.Version != 0 || reports[0].cause != smf.ResourcesNotAllocated {
		t.Fatalf("reports %+v, want version 1 failed with version 0 still active (TS 29.212 §4.5.12)", reports)
	}

	pushRules(t, s, ref, movedVoiceRule())
	requireModificationCount(t, mmeCb, 1)

	if d := mmeCb.dedicatedDeactivations(); len(d) != 0 {
		t.Fatalf("deactivations %v, want the bearer kept with the enforced rule", d)
	}

	if ports := bearerUplinkPorts(t, upf); !slices.Equal(ports, []uint16{50000}) {
		t.Fatalf("bearer uplink ports %v, want the enforced rule's 50000", ports)
	}
}

func TestRejectedRemovalIsRetriedAfterABackoff(t *testing.T) {
	s, pcf, _, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitDedicatedModifications(t, mmeCb, 1)
	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

	pushRules(t, s, ref, secondCallRule())
	waitDedicatedModifications(t, mmeCb, 2)

	s.SetDedicatedAwaitLimitForTest(50 * time.Millisecond)
	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, false)

	requireModificationCount(t, mmeCb, 2)

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none: a removal does not fail (TS 29.212 §4.5.12)", r)
	}

	if retry := waitDedicatedModifications(t, mmeCb, 3)[2]; retry.Operation != models.TFTDeleteFilters {
		t.Fatalf("retry %+v, want the deletion sent again", retry)
	}
}

func TestRejectedLaterStepIsReportedOnce(t *testing.T) {
	s, pcf, _, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	second := secondCallRule()
	second.Version = 2

	pushRules(t, s, ref, movedVoiceRule(), second)

	replace := waitDedicatedModifications(t, mmeCb, 1)[0]
	if replace.Operation != models.TFTReplaceFilters || replace.QoSChanged {
		t.Fatalf("first step %+v, want the replacement alone, without the second call's QoS", replace)
	}

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

	add := waitDedicatedModifications(t, mmeCb, 2)[1]
	if add.Operation != models.TFTAddFilters || add.GBR.Downlink.Bps() != 82000 {
		t.Fatalf("second step %+v, want the addition with an 82 kbps GBR", add)
	}

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, false)

	if r := pcf.reportedRules(); !slices.Equal(r, []string{"af;2#1"}) {
		t.Fatalf("reported rules %v, want only the second call's", r)
	}

	requireModificationCount(t, mmeCb, 2)
}

func TestRulesChangedDuringActivationAreModifiedAfterIt(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule())
	pushRules(t, s, ref, voiceRule(), secondCallRule())

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d activations, want one", n)
	}

	requireModificationCount(t, mmeCb, 0)

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	if add := waitDedicatedModifications(t, mmeCb, 1)[0]; add.Operation != models.TFTAddFilters {
		t.Fatalf("modification %+v, want the second call added after the activation", add)
	}

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, false)

	reports := pcf.reportedFailures()
	if len(reports) != 1 || reports[0].Rule.ID != "af;2#1" || reports[0].Active != nil {
		t.Fatalf("reports %+v, want the second call failed with nothing active", reports)
	}

	requireModificationCount(t, mmeCb, 1)
}

func TestBearerLostDuringModificationReportsEveryRule(t *testing.T) {
	s, pcf, _, mmeCb, ref := activeVoiceSession(t)

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitDedicatedModifications(t, mmeCb, 1)

	s.DedicatedBearerReleased(context.Background(), ref, dedicatedUplinkTEID)

	reports := pcf.reportedFailures()

	var ids []string
	for _, r := range reports {
		ids = append(ids, r.Rule.ID)

		if r.cause != smf.BearerReleased || r.Active != nil {
			t.Fatalf("report %+v, want BearerReleased with nothing active (TS 29.214 §4.4.6.2)", r)
		}
	}

	slices.Sort(ids)

	if !slices.Equal(ids, []string{"af;1#1", "af;2#1"}) {
		t.Fatalf("reported rules %v, want the installed and the pending call", ids)
	}
}

func TestRadioEndpointChangeKeepsTheModification(t *testing.T) {
	s, _, upf, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitDedicatedModifications(t, mmeCb, 1)

	if err := s.DedicatedBearerMoved(ctx, ref, dedicatedUplinkTEID, models.FTEID{TEID: 0x77, Addr: netip.MustParseAddr("10.3.0.7")}); err != nil {
		t.Fatalf("DedicatedBearerMoved: %v", err)
	}

	requireModificationCount(t, mmeCb, 1)

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

	if n := bearerUplinkFilters(t, upf); n != 2 {
		t.Fatalf("bearer uplink PDR has %d filters after the accept, want both calls' 2", n)
	}

	requireModificationCount(t, mmeCb, 1)
}

func TestARPChangeOfEveryRuleModifiesTheBearerInPlace(t *testing.T) {
	s, _, _, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	raised := voiceRule()
	raised.Version = 1
	raised.ARP.PriorityLevel = 1

	pushRules(t, s, ref, raised)

	mod := waitDedicatedModifications(t, mmeCb, 1)[0]
	if mod.ARP == nil || mod.ARP.PriorityLevel != 1 || mod.QoSChanged || mod.Operation != models.TFTNoChange {
		t.Fatalf("modification %+v, want only the ARP changed (TS 29.213 §5.4)", mod)
	}

	if n, d := len(mmeCb.dedicatedActivations()), mmeCb.dedicatedDeactivations(); n != 1 || len(d) != 0 {
		t.Fatalf("%d activations and deactivations %v, want the bearer kept", n, d)
	}

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

	second := secondCallRule()
	second.ARP.PriorityLevel = 1
	pushRules(t, s, ref, raised, second)

	if add := waitDedicatedModifications(t, mmeCb, 2)[1]; add.Operation != models.TFTAddFilters || add.ARP != nil {
		t.Fatalf("modification %+v, want the second call added to the re-keyed bearer", add)
	}

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d activations, want none for the bearer's new ARP", n)
	}
}
