// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/s1ap"
)

// pagingActive reports whether a paging supervision timer is armed for the UE.
func (m *MME) pagingActive(ue *UeContext) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ue.pagingTimer.Active()
}

func TestUEIdentityIndex(t *testing.T) {
	// IMSI mod 1024 (TS 36.304 §7.1).
	if got := ueIdentityIndex("001010000000444"); got != uint16(1010000000444%1024) {
		t.Fatalf("ueIdentityIndex = %d, want %d", got, uint16(1010000000444%1024))
	}
}

// TestBuildPaging checks the MME assembles a Paging with the UE's S-TMSI, the
// IMSI-derived index, the PS domain, and the operator's tracking area, and that
// it marshals to a valid S1AP Paging PDU.
func TestBuildPaging(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	paging, err := m.buildPaging(context.Background(), ue)
	if err != nil {
		t.Fatalf("buildPaging: %v", err)
	}

	if paging.STMSI.MTMSI != ue.mtmsi {
		t.Fatalf("S-TMSI M-TMSI = %#x, want %#x", paging.STMSI.MTMSI, ue.mtmsi)
	}

	if paging.UEIdentityIndexValue != ueIdentityIndex(ue.imsi) {
		t.Fatalf("UE identity index = %d, want %d", paging.UEIdentityIndexValue, ueIdentityIndex(ue.imsi))
	}

	if paging.CNDomain != s1ap.CNDomainPS {
		t.Fatalf("CN domain = %d, want PS", paging.CNDomain)
	}

	if len(paging.TAIList) != 1 {
		t.Fatalf("TAI list length = %d, want 1", len(paging.TAIList))
	}

	b, err := paging.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if im, ok := pdu.(*s1ap.InitiatingMessage); !ok || im.ProcedureCode != s1ap.ProcPaging {
		t.Fatalf("expected Paging InitiatingMessage, got %T", pdu)
	}
}

// TestPageNoENBs checks Page builds and broadcasts without error when a UE is
// idle (and no eNBs are connected, so the broadcast is a no-op).
func TestPageNoENBs(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsi); err != nil {
		t.Fatalf("Page: %v", err)
	}
}

func TestPageUnknownIMSI(t *testing.T) {
	m := newTestMME(t)

	if err := m.Page(context.Background(), "001010000000999"); err == nil {
		t.Fatal("Page should error for an unknown IMSI")
	}
}

// TestPagingRetransmitsThenAbandons confirms the MME supervises paging with a
// timer (T3413): when the UE never responds, the Paging is retransmitted a
// bounded number of times and the procedure is then abandoned (TS 24.301
// §5.6.2).
func TestPagingRetransmitsThenAbandons(t *testing.T) {
	m := newTestMME(t)
	m.pagingTimeout = 5 * time.Millisecond
	m.pagingMaxRetransmit = 2

	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsi); err != nil {
		t.Fatalf("Page: %v", err)
	}

	if !m.pagingActive(ue) {
		t.Fatal("paging not supervised after Page")
	}

	deadline := time.Now().Add(time.Second)
	for m.pagingActive(ue) {
		if time.Now().After(deadline) {
			t.Fatal("paging procedure not abandoned after the retransmission budget")
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// TestPagingStoppedOnReconnect confirms the paging supervision is cancelled when
// the UE returns from ECM-IDLE on a new S1 connection (the paging response).
func TestPagingStoppedOnReconnect(t *testing.T) {
	m := newTestMME(t)
	m.pagingTimeout = time.Hour // long enough not to fire during the test

	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsi); err != nil {
		t.Fatalf("Page: %v", err)
	}

	if !m.pagingActive(ue) {
		t.Fatal("paging not supervised after Page")
	}

	m.EstablishS1Connection(ue, &captureConn{}, 9)

	if m.pagingActive(ue) {
		t.Fatal("paging supervision not stopped when the UE reconnected")
	}
}
