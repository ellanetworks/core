// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

// TS 24.301 §5.5.1.2.7
func TestPlainAttachDoesNotSupersedeRegisteredVictimPreAuth(t *testing.T) {
	m := newTestMME(t)
	victim, _ := securedUE(t, m)

	attacker := m.NewUe(&captureConn{}, 8)
	m.SetIMSI(attacker, victim.IMSI())

	got, ok := m.LookupUeByIMSI(victim.IMSI())
	if !ok || got != victim {
		t.Fatal("an unauthenticated attach must not supersede the registered victim before authentication (TS 24.301 §5.5.1.2.7 f)")
	}

	if victim.EMMState() != EMMRegistered {
		t.Fatal("victim must remain EMM-REGISTERED")
	}

	if err := m.CommitUEIdentity(context.Background(), attacker, MintAuthProofForAttachCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	if got, _ := m.LookupUeByIMSI(victim.IMSI()); got != attacker {
		t.Fatal("after commit, the authenticated attach must supersede the prior context")
	}
}

func TestEstablishS1ConnectionMarksSecureExchange(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	establishResumeForTest(m, ue, &captureConn{}, 9)

	if !ue.Conn().secureExchangeEstablished {
		t.Fatal("a verified resume must establish secure exchange on the new connection (TS 24.301 §4.4.4.3)")
	}
}

func establishResumeForTest(m *MME, ue *UeContext, conn S1APWriter, enbUEID s1ap.ENBUES1APID) {
	c := m.NewUeConn(conn, enbUEID)
	m.AttachUeConn(ue, c)
	c.MarkSecureExchangeEstablished()
}

func TestAttachUeConn_ClearsEPSPagingSuppression(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Pdns = map[uint8]*PdnConnection{
		5: {Ebi: 5},
		6: {Ebi: 6},
	}

	m.AttachUeConn(ue, m.NewUeConn(&captureConn{}, 9))

	fake := m.Session.(*fakeSessionManager)
	if fake.clearSuppressionCalls != 2 {
		t.Fatalf("clear-suppression calls = %d, want 2 (one per PDN)", fake.clearSuppressionCalls)
	}
}

func TestAbandonPaging_SuppressesAllPDNs(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Pdns = map[uint8]*PdnConnection{
		5: {Ebi: 5},
		6: {Ebi: 6},
	}

	m.abandonPaging(ue)

	fake := m.Session.(*fakeSessionManager)
	if fake.suppressCalls != 2 {
		t.Fatalf("suppress calls = %d, want 2 (one per PDN)", fake.suppressCalls)
	}
}
