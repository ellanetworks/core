// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TestGUTIReallocationCommitsOnComplete verifies the old M-TMSI stays resolvable until
// GUTI REALLOCATION COMPLETE commits the new GUTI and frees the old one (TS 24.301 §5.4.1).
func TestGUTIReallocationCommitsOnComplete(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	// Give the UE an initial committed GUTI so the reallocation has an old M-TMSI.
	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	gid, code := m.MmeIdentity()

	first, err := m.ReallocateGUTI(context.Background(), ue, plmn, gid, code)
	if err != nil {
		t.Fatal(err)
	}

	m.CommitGUTIRealloc(ue)

	before := len(cc.sent)

	m.SendGUTIReallocationCommand(context.Background(), ue)

	if len(cc.sent) != before+1 {
		t.Fatalf("expected one GUTI Reallocation Command downlink, got %d", len(cc.sent)-before)
	}

	if _, ok := m.LookupUeByMTMSI(first.MTMSI); !ok {
		t.Fatal("old M-TMSI must stay resolvable until the UE acknowledges")
	}

	complete, err := (&eps.GUTIReallocationComplete{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleGUTIReallocationComplete(context.Background(), m, ue, complete)

	if _, ok := m.LookupUeByMTMSI(first.MTMSI); ok {
		t.Fatal("old M-TMSI still resolvable after GUTI Reallocation Complete")
	}
}
