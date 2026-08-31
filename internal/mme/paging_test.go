// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func (m *MME) pagingActive(ue *UeContext) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ue.pagingTimer.Active()
}

func TestUEIdentityIndex(t *testing.T) {
	if got := ueIdentityIndex("001010000000444"); got != uint16(1010000000444%1024) {
		t.Fatalf("ueIdentityIndex = %d, want %d", got, uint16(1010000000444%1024))
	}
}

func TestBuildPaging(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	ue.RadioCapabilityForPaging = []byte{0xaa, 0xbb, 0xcc}

	paging, err := m.buildPaging(t.Context(), ue)
	if err != nil {
		t.Fatalf("buildPaging: %v", err)
	}

	if !bytes.Equal(paging.UERadioCapabilityForPaging, ue.RadioCapabilityForPaging) {
		t.Fatalf("paging UE Radio Capability for Paging = %x, want %x", paging.UERadioCapabilityForPaging, ue.RadioCapabilityForPaging)
	}

	if uint32(paging.STMSI.MTMSI) != ue.Tmsi().Uint32() {
		t.Fatalf("S-TMSI M-TMSI = %#x, want %#x", paging.STMSI.MTMSI, ue.Tmsi().Uint32())
	}

	if paging.UEIdentityIndexValue == nil || *paging.UEIdentityIndexValue != ueIdentityIndex(ue.imsiOrEmpty()) {
		t.Fatalf("UE identity index = %v, want %d", paging.UEIdentityIndexValue, ueIdentityIndex(ue.imsiOrEmpty()))
	}

	if paging.CNDomain == nil || *paging.CNDomain != s1ap.CNDomainPS {
		t.Fatalf("CN domain = %v, want PS", paging.CNDomain)
	}

	if len(paging.TAIList) != 1 {
		t.Fatalf("TAI list length = %d, want 1", len(paging.TAIList))
	}

	if uint16(paging.TAIList[0].TAC) != 1 {
		t.Fatalf("paging TAI TAC = %d, want 1 (served)", uint16(paging.TAIList[0].TAC))
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

// TS 23.003 §2.4
func TestBuildPagingOldMTMSISentinel(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	ue.SetTmsiForTest(0x1234)
	ue.SetOldTmsiForTest(0)

	paging, err := m.buildPaging(t.Context(), ue)
	if err != nil {
		t.Fatalf("buildPaging: %v", err)
	}

	if paging.STMSI.MTMSI != 0 {
		t.Fatalf("paged on M-TMSI %#x, want 0 (the old M-TMSI the UE still listens on)", paging.STMSI.MTMSI)
	}

	ue.SetOldTmsiForTest(0xFFFFFFFF)

	paging, err = m.buildPaging(t.Context(), ue)
	if err != nil {
		t.Fatalf("buildPaging: %v", err)
	}

	if paging.STMSI.MTMSI != 0x1234 {
		t.Fatalf("paged on M-TMSI %#x, want 0x1234 (the current M-TMSI)", paging.STMSI.MTMSI)
	}
}

func TestPageNoENBs(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsiOrEmpty()); err != nil {
		t.Fatalf("Page: %v", err)
	}
}

// TS 23.401 §5.3.4
func TestPageFiltersByServedTAI(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	p := plmn

	served := &captureConn{}
	m.IndexRadioForTest(served, []SupportedTAI{{Tai: models.Tai{PlmnID: &p, Tac: "000001"}}})

	foreign := &captureConn{}
	m.IndexRadioForTest(foreign, []SupportedTAI{{Tai: models.Tai{PlmnID: &p, Tac: "0000ff"}}})

	if err := m.Page(context.Background(), ue.imsiOrEmpty()); err != nil {
		t.Fatalf("Page: %v", err)
	}

	if got := served.count(); got != 1 {
		t.Fatalf("eNB serving the network TAC received %d pages, want 1", got)
	}

	if got := foreign.count(); got != 0 {
		t.Fatalf("eNB serving only a foreign TAC received %d pages, want 0", got)
	}
}

func TestPageUnknownIMSI(t *testing.T) {
	m := newTestMME(t)

	if err := m.Page(context.Background(), "001010000000999"); err == nil {
		t.Fatal("Page should error for an unknown IMSI")
	}
}

func TestPagingRetransmitsThenAbandons(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = 5 * time.Millisecond
	m.pagingCfg.MaxRetryTimes = 2

	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsiOrEmpty()); err != nil {
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

func TestPagingStoppedOnReconnect(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)

	if err := m.Page(context.Background(), ue.imsiOrEmpty()); err != nil {
		t.Fatalf("Page: %v", err)
	}

	if !m.pagingActive(ue) {
		t.Fatal("paging not supervised after Page")
	}

	establishResumeForTest(m, ue, &captureConn{}, 9)

	if m.pagingActive(ue) {
		t.Fatal("paging supervision not stopped when the UE reconnected")
	}
}
