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

func ruleGates(t *testing.T, upf *fakeUPF) map[uint32]models.GateStatus {
	t.Helper()

	m := lastModify(t, upf)
	gates := make(map[uint32]models.GateStatus)

	for _, p := range m.UpdatePDRs {
		if p.QERID < 256 {
			continue
		}

		i := slices.IndexFunc(m.UpdateQERs, func(q models.QER) bool { return q.QERID == p.QERID })
		if i < 0 || m.UpdateQERs[i].GateStatus == nil {
			t.Fatalf("PDR %d names QER %d, which carries no gate", p.PDRID, p.QERID)
		}

		gates[p.QERID] = *m.UpdateQERs[i].GateStatus
	}

	return gates
}

func heldRule(r smf.PCCRule) smf.PCCRule {
	r.Version++
	r.Gate = models.GateStatus{DLGate: models.GateClose}

	return r
}

func TestHoldGatesOnlyTheHeldCallWithoutSignalling(t *testing.T) {
	s, pcf, upf, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitDedicatedModifications(t, mmeCb, 1)
	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, models.ModificationCommitted)

	pushRules(t, s, ref, heldRule(voiceRule()), secondCallRule())
	requireModificationCount(t, mmeCb, 1)

	gates := ruleGates(t, upf)
	closed, open := 0, 0

	for _, g := range gates {
		switch g {
		case models.GateStatus{DLGate: models.GateClose}:
			closed++
		case models.GateStatus{}:
			open++
		}
	}

	if len(gates) != 2 || closed != 1 || open != 1 {
		t.Fatalf("rule gates %+v, want the held call's downlink closed and the other call open (TS 29.244 §5.4.3)", gates)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none", r)
	}

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	requireModificationCount(t, mmeCb, 1)

	for q, g := range ruleGates(t, upf) {
		if g != (models.GateStatus{}) {
			t.Fatalf("QER %d gate %+v after resume, want open", q, g)
		}
	}

	if d := mmeCb.dedicatedDeactivations(); len(d) != 0 {
		t.Fatalf("deactivations %v, want the bearer kept through hold and resume", d)
	}
}

func TestPartlyCarriedRuleKeepsItsRatesAndGate(t *testing.T) {
	s, _, upf, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	moved := voiceRule()
	moved.Version = 1
	moved.Filters = append(slices.Clone(moved.Filters), moved.Filters[0])
	moved.Filters[0].LocalPort = 50010
	moved.Filters[2].RemotePort = 49010

	upf.mu.Lock()
	from := len(upf.modifyCalls)
	upf.mu.Unlock()

	pushRules(t, s, ref, moved)
	waitDedicatedModifications(t, mmeCb, 1)

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, models.ModificationCommitted)
	waitDedicatedModifications(t, mmeCb, 2)

	upf.mu.Lock()
	calls := slices.Clone(upf.modifyCalls[from:])
	upf.mu.Unlock()

	for _, m := range calls {
		for _, q := range m.UpdateQERs {
			if q.QERID < 256 {
				continue
			}

			if q.MBR == nil || q.MBR.ULMBR == 0 || q.MBR.DLMBR == 0 || *q.GateStatus != (models.GateStatus{}) {
				t.Fatalf("rule QER %+v, want the call's rate and open gates while its filters are replaced step by step", q)
			}
		}
	}
}

func ringingRule() smf.PCCRule {
	r := voiceRule()
	r.Gate = models.GateStatus{ULGate: models.GateClose, DLGate: models.GateClose}

	return r
}

func answeredRule() smf.PCCRule {
	r := voiceRule()
	r.Version = 1

	rate := models.BitRateFromBps(30000)
	r.MBR, r.GBR = models.Ambr{Uplink: rate, Downlink: rate}, models.Ambr{Uplink: rate, Downlink: rate}

	return r
}

func reopenedRule() smf.PCCRule {
	r := voiceRule()
	r.Version = 2

	return r
}

func requireOpenGates(t *testing.T, upf *fakeUPF) {
	t.Helper()

	gates := ruleGates(t, upf)
	if len(gates) == 0 {
		t.Fatal("no rule QER")
	}

	for q, g := range gates {
		if g != (models.GateStatus{}) {
			t.Fatalf("QER %d gate %+v, want open", q, g)
		}
	}
}

func TestGateReopenedOnTheEnforcedBearerQoSNeedsNoSignalling(t *testing.T) {
	s, pcf, upf, mmeCb, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, ringingRule())

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	pushRules(t, s, ref, answeredRule())
	waitDedicatedModifications(t, mmeCb, 1)

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, models.ModificationFailed)

	reports := pcf.reportedFailures()
	if len(reports) != 1 || reports[0].Active == nil || !reports[0].Active.Equal(ringingRule()) {
		t.Fatalf("reports %+v, want the answered rule failed with the ringing rule still active (TS 29.212 §4.5.12)", reports)
	}

	pushRules(t, s, ref, reopenedRule())
	requireModificationCount(t, mmeCb, 1)
	requireOpenGates(t, upf)

	if r := pcf.reportedFailures(); len(r) != 1 {
		t.Fatalf("reports %+v, want only the failed modification", r)
	}
}

func TestGateReopenedOnTheEnforcedQoSFlowNeedsNoSignalling(t *testing.T) {
	s, pcf, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, ringingRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref, answeredRule())
	waitFlowModifications(t, amfCb, 2)
	ueAnswers(t, s, ref, false)
	ranAnswers(t, s, ref, nil, []uint8{voiceQFI})

	reports := pcf.reportedFailures()
	if len(reports) != 1 || reports[0].Active == nil || !reports[0].Active.Equal(ringingRule()) {
		t.Fatalf("reports %+v, want the answered rule failed with the ringing rule still active (TS 29.512 RES_ALLO_FAIL)", reports)
	}

	pushRules(t, s, ref, reopenedRule())
	time.Sleep(100 * time.Millisecond)

	if n := len(amfCb.modifications()); n != 2 {
		t.Fatalf("%d QoS flow modifications, want the gate opened without another", n)
	}

	requireOpenGates(t, upf)

	if r := pcf.reportedFailures(); len(r) != 1 {
		t.Fatalf("reports %+v, want only the failed modification", r)
	}
}
