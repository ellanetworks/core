// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/binary"
	"testing"
)

// TS 24.301 §5.4.1
func TestGUTIReallocationCommitsOnComplete(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	gid, code, err := m.MmeIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}

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

	if _, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(first.GUTI.TMSI[:])); !ok {
		t.Fatal("old M-TMSI must stay resolvable until the UE acknowledges")
	}

	handleGUTIReallocationComplete(context.Background(), m, ue, ue.Conn())

	if _, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(first.GUTI.TMSI[:])); ok {
		t.Fatal("old M-TMSI still resolvable after GUTI Reallocation Complete")
	}
}
