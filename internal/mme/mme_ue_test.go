// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

// TestPlainAttachDoesNotSupersedeRegisteredVictimPreAuth asserts TS 24.301
// §4.4.4.3: an unauthenticated attach citing a registered subscriber's
// (cleartext) IMSI must not tear down that subscriber's context. The prior
// context is superseded only once the new attach is authenticated and accepted.
func TestPlainAttachDoesNotSupersedeRegisteredVictimPreAuth(t *testing.T) {
	m := newTestMME(t)
	victim, _ := securedUE(t, m)

	// A fresh, not-yet-authenticated attach context claiming the victim's IMSI.
	attacker := m.NewUe(&captureConn{}, 8)
	m.SetIMSI(attacker, victim.IMSI())

	got, ok := m.LookupUeByIMSI(victim.IMSI())
	if !ok || got != victim {
		t.Fatal("an unauthenticated attach must not supersede the registered victim before authentication (TS 24.301 §4.4.4.3)")
	}

	if victim.EMMState() != EMMRegistered {
		t.Fatal("victim must remain EMM-REGISTERED")
	}

	// Once the new attach is authenticated and accepted, it supersedes the prior
	// context (a re-attach), so the subscriber maps to exactly one context.
	m.CommitUEIdentity(context.Background(), attacker, MintAuthProofForAttachCommit())

	if got, _ := m.LookupUeByIMSI(victim.IMSI()); got != attacker {
		t.Fatal("after commit, the authenticated attach must supersede the prior context")
	}
}

// TestEstablishS1ConnectionMarksSecureExchange asserts the per-connection
// §4.4.4.3 flag is set when a UE resumes on a new connection — a resume only
// reaches establishS1Connection after its message was integrity-verified.
func TestEstablishS1ConnectionMarksSecureExchange(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	establishResumeForTest(m, ue, &captureConn{}, 9)

	if !ue.Conn().secureExchangeEstablished {
		t.Fatal("a verified resume must establish secure exchange on the new connection (TS 24.301 §4.4.4.3)")
	}
}

// TestVerifiedMessageMarksSecureExchange asserts a successfully integrity-checked
// message establishes secure exchange on a connection that did not have it yet
// (the fresh-attach case, where the flag is set when SMC Complete verifies).

// establishResumeForTest binds a UE returning from ECM-IDLE to a fresh verified
// S1 connection, the resume primitives HandleServiceRequest uses (NewUeConn +
// AttachUeConn + mark secure exchange).
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
