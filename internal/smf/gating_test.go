// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"slices"
	"testing"

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
	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

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

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)
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
