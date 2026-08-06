// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
)

func TestSessionTransferredDropsPDNWithoutReleasing(t *testing.T) {
	m := newTestMME(t)
	sm := &fakeSessionManager{}
	m.Session = sm

	ue := m.NewUe(&captureConn{}, 7)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)
	ue.ForceStateForTest(EMMRegistered)

	moved := ue.EnsurePDN(DefaultERABID)
	moved.SessionRef = "moved-to-5gs"

	kept := ue.EnsurePDN(DefaultERABID + 1)
	kept.SessionRef = "still-on-eps"

	m.SessionTransferred(context.Background(), testSubscriber.IMSI, moved.Ebi)

	if sm.released {
		t.Error("released the anchor session, want no release")
	}

	if m.LookupPDN(ue, moved.Ebi) != nil {
		t.Error("PDN connection for the moved session is still present")
	}

	if m.LookupPDN(ue, kept.Ebi) == nil {
		t.Error("an unrelated PDN connection was dropped")
	}

	m.ReleaseAllSessions(context.Background(), ue)

	if want := []string{"still-on-eps"}; !equalStrings(sm.releasedRefs, want) {
		t.Errorf("detach released %v, want %v", sm.releasedRefs, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
