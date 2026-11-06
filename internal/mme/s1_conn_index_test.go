// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func assertConnIndexConsistent(t *testing.T, m *MME) {
	t.Helper()

	m.mu.RLock()
	defer m.mu.RUnlock()

	byKey := make(map[s1ConnKey]int)
	byAssoc := make(map[S1APWriter]int)

	for _, c := range m.conns {
		key := s1ConnKey{conn: c.Conn(), enbUEID: c.ENBUES1APID()}

		if _, ok := m.connIndex.byKey[key][c]; !ok {
			t.Fatalf("connection %d is not indexed under its eNB-UE-S1AP-ID %d", c.MMEUES1APID, key.enbUEID)
		}

		if _, ok := m.connIndex.byAssoc[key.conn][c]; !ok {
			t.Fatalf("connection %d is not indexed under its association", c.MMEUES1APID)
		}

		byKey[key]++
		byAssoc[key.conn]++
	}

	if len(byKey) != len(m.connIndex.byKey) || len(byAssoc) != len(m.connIndex.byAssoc) {
		t.Fatalf("index holds stale keys: %d/%d by key, %d/%d by association",
			len(m.connIndex.byKey), len(byKey), len(m.connIndex.byAssoc), len(byAssoc))
	}

	for key, set := range m.connIndex.byKey {
		if len(set) != byKey[key] {
			t.Fatalf("index holds %d connections under eNB-UE-S1AP-ID %d, want %d", len(set), key.enbUEID, byKey[key])
		}
	}

	for conn, set := range m.connIndex.byAssoc {
		if len(set) != byAssoc[conn] {
			t.Fatalf("index holds %d connections on an association, want %d", len(set), byAssoc[conn])
		}
	}
}

func TestConnIndexFollowsConnectionLifecycle(t *testing.T) {
	m := newTestMME(t)
	ctx := t.Context()

	source, target := &captureConn{}, &captureConn{}

	stale := m.NewUe(ctx, source, 7)
	other := m.NewUe(ctx, source, 8)
	bare := m.NewUeConn(source, 7)
	staleID := stale.Conn().MMEUES1APID

	assertConnIndexConsistent(t, m)

	if got := len(m.ConnsOnConn(source)); got != 3 {
		t.Fatalf("ConnsOnConn: got %d connections, want 3", got)
	}

	enb7 := s1ap.ENBUES1APID(7)

	if !m.ReleaseConnsWithAPIDs(ctx, source, nil, &enb7, "test") {
		t.Fatal("ReleaseConnsWithAPIDs matched no connection")
	}

	assertConnIndexConsistent(t, m)

	if _, ok := m.LookupUe(staleID); ok {
		t.Fatal("ReleaseConnsWithAPIDs kept the UE holding the eNB-UE-S1AP-ID")
	}

	if _, ok := m.conns[uint32(bare.MMEUES1APID)]; ok {
		t.Fatal("ReleaseConnsWithAPIDs kept a bare connection holding the eNB-UE-S1AP-ID")
	}

	if _, ok := m.LookupUe(other.Conn().MMEUES1APID); !ok {
		t.Fatal("ReleaseConnsWithAPIDs removed a UE on another eNB-UE-S1AP-ID")
	}

	if _, ok := m.CommitPathSwitch(other, other.Conn(), target, 9, [32]byte{}, 0); !ok {
		t.Fatal("CommitPathSwitch did not commit")
	}

	assertConnIndexConsistent(t, m)

	enb8, enb9 := s1ap.ENBUES1APID(8), s1ap.ENBUES1APID(9)

	if got := m.ConnsForConnectionList(source, []s1ap.UEAssociatedLogicalS1ConnectionItem{{ENBUES1APID: &enb8}}); len(got) != 0 {
		t.Fatalf("reset on the old association matched %d connections after a path switch", len(got))
	}

	got := m.ConnsForConnectionList(target, []s1ap.UEAssociatedLogicalS1ConnectionItem{{ENBUES1APID: &enb9}})
	if len(got) != 1 || got[0] != other.Conn() {
		t.Fatalf("reset on the new association did not resolve the switched connection: %v", got)
	}

	if got := len(m.ConnsOnConn(source)); got != 0 {
		t.Fatalf("ConnsOnConn(source): got %d connections, want none", got)
	}

	m.ReleaseConnsWithAPIDs(ctx, target, nil, &enb9, "test")
	assertConnIndexConsistent(t, m)

	if len(m.conns) != 0 || len(m.connIndex.byKey) != 0 || len(m.connIndex.byAssoc) != 0 {
		t.Fatalf("connections or index entries left behind: %d conns, %d keys, %d associations",
			len(m.conns), len(m.connIndex.byKey), len(m.connIndex.byAssoc))
	}
}
