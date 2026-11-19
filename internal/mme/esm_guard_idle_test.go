// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func secondPDNWithShortESMGuard(t *testing.T) (*MME, *UeContext, *PdnConnection) {
	t.Helper()

	m := newTestMME(t)
	m.SetESMGuardConfigForTest(5*time.Millisecond, 1)

	ue, _ := connectedBearerUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	return m, ue, addSecondPDN(ue)
}

func TestT3495ExpiryWhileIdleReleasesThePDN(t *testing.T) {
	m, ue, ims := secondPDNWithShortESMGuard(t)

	if err := m.ReactivateEPSBearer(context.Background(), ue.imsiOrEmpty(), ims.Ebi, models.ReleasePolicyChanged); err != nil {
		t.Fatalf("ReactivateEPSBearer: %v", err)
	}

	m.FreeUeConn(t.Context(), ue)

	eventually(t, 2*time.Second, func() bool { return m.LookupPDN(ue, ims.Ebi) == nil })

	if existing := m.FindPDNByAPN(ue, "ims"); existing != nil {
		t.Errorf("APN ims is still claimed by PDN connection %q after T3495 exhausted while the UE was idle", existing.SessionRef)
	}
}

func TestT3485ExpiryWhileIdleReleasesThePDN(t *testing.T) {
	m, ue, ims := secondPDNWithShortESMGuard(t)

	m.BeginPDNActivation(context.Background(), ue, ims, 3, []byte{0xd0}, []byte{0x02})
	m.FreeUeConn(t.Context(), ue)

	eventually(t, 2*time.Second, func() bool { return m.LookupPDN(ue, ims.Ebi) == nil })

	if !m.Session.(*fakeSessionManager).wasReleased() {
		t.Error("the SMF session was not released after T3485 exhausted while the UE was idle")
	}
}
