// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func modifyingAdditionalPDN(t *testing.T, m *MME) (*UeContext, *PdnConnection) {
	t.Helper()

	ue, _ := connectedBearerUE(t, m)

	p := ue.EnsurePDN(6)
	p.Apn = "ims"
	p.SessionRef = "ref-ims"
	p.Qci = 5
	p.Arp = 1

	return ue, p
}

func modifyQoS(t *testing.T, m *MME, ue *UeContext, ebi uint8) {
	t.Helper()

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), ebi, models.EPSBearerModification{QoS: &models.EPSBearerQoS{QCI: 6, ARP: 1}}); err != nil {
		t.Fatal(err)
	}
}

func waitForOutcome(m *MME) []bearerModificationOutcome {
	fake := m.Session.(*fakeSessionManager)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := fake.outcomes(); len(got) > 0 {
			return got
		}

		time.Sleep(time.Millisecond)
	}

	return nil
}

func waitForPendingModifyCleared(ue *UeContext, p *PdnConnection) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ue.mu.Lock()
		done := p.Modifying == nil
		ue.mu.Unlock()

		if done {
			return true
		}

		time.Sleep(time.Millisecond)
	}

	return false
}

// TS 24.301 §6.4.2.5.
func TestModifyBearerGuardAbortClearsTheModifiedPDN(t *testing.T) {
	m := newTestMME(t)
	ue, p := modifyingAdditionalPDN(t, m)

	m.SetESMGuardConfigForTest(20*time.Millisecond, 0)
	modifyQoS(t, m, ue, p.Ebi)

	want := []bearerModificationOutcome{{ref: "ref-ims", accepted: false}}
	if got := waitForOutcome(m); !slices.Equal(got, want) {
		t.Fatalf("SMF told %+v, want %+v", got, want)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if p.Modifying != nil || p.Qci != 5 {
		t.Fatalf("aborted modification left Modifying %+v and QCI %d, want nil and 5", p.Modifying, p.Qci)
	}
}

func TestModifyBearerGuardAbortLeavesOtherPDNsIntact(t *testing.T) {
	m := newTestMME(t)
	ue, p := modifyingAdditionalPDN(t, m)

	def := testPDN(ue)
	pending := &models.EPSBearerModification{QoS: &models.EPSBearerQoS{QCI: 9, ARP: 1}}
	def.Modifying = pending

	m.SetESMGuardConfigForTest(20*time.Millisecond, 0)
	modifyQoS(t, m, ue, p.Ebi)

	if !waitForPendingModifyCleared(ue, p) {
		t.Fatal("additional PDN's modification not cleared by its own guard abort")
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if def.Modifying != pending {
		t.Fatalf("default bearer modification = %+v after an additional PDN's guard aborted, want it untouched", def.Modifying)
	}
}
