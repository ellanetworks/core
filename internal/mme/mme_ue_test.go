// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func initiatingValue(t *testing.T, b []byte) []byte {
	t.Helper()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok {
		t.Fatalf("expected InitiatingMessage, got %T", pdu)
	}

	return im.Value
}

func TestUplinkNASTransportUnknownUE(t *testing.T) {
	m := newTestMME(t)

	uplink := &s1ap.UplinkNASTransport{
		MMEUES1APID: 999,
		ENBUES1APID: 7,
		NASPDU:      s1ap.NASPDU{0x07, 0x56},
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}

	b, err := uplink.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// An unknown MME-UE-S1AP-ID is answered with an Error Indication, not
	// silently dropped, and no context is created (TS 36.413).
	conn := &captureConn{}
	m.handleUplinkNASTransport(context.Background(), conn, initiatingValue(t, b))

	if _, ok := m.lookupUe(999); ok {
		t.Fatal("unexpected UE context for unknown MME-UE-S1AP-ID")
	}

	if len(conn.sent) != 1 {
		t.Fatalf("expected one Error Indication, got %d", len(conn.sent))
	}

	ind := parseOutboundErrorIndication(t, conn.sent[0])
	if ind.MMEUES1APID == nil || *ind.MMEUES1APID != 999 || ind.ENBUES1APID == nil || *ind.ENBUES1APID != 7 {
		t.Fatalf("expected the received AP-ID pair (999, 7), got (%v, %v)", ind.MMEUES1APID, ind.ENBUES1APID)
	}

	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}

// TestPlainAttachDoesNotSupersedeRegisteredVictimPreAuth asserts TS 24.301
// §4.4.4.3: an unauthenticated attach citing a registered subscriber's
// (cleartext) IMSI must not tear down that subscriber's context. The prior
// context is superseded only once the new attach is authenticated and accepted.
func TestPlainAttachDoesNotSupersedeRegisteredVictimPreAuth(t *testing.T) {
	m := newTestMME(t)
	victim, _ := securedUE(t, m)

	// A fresh, not-yet-authenticated attach context claiming the victim's IMSI.
	attacker := m.newUe(&captureConn{}, 8)
	m.setIMSI(attacker, victim.imsi)

	got, ok := m.lookupUeByIMSI(victim.imsi)
	if !ok || got != victim {
		t.Fatal("an unauthenticated attach must not supersede the registered victim before authentication (TS 24.301 §4.4.4.3)")
	}

	if victim.emmState.load() != EMMRegistered {
		t.Fatal("victim must remain EMM-REGISTERED")
	}

	// Once the new attach is authenticated and accepted, it supersedes the prior
	// context (a re-attach), so the subscriber maps to exactly one context.
	m.commitUEIdentity(attacker, MintAuthProofForAttachCommit())

	if got, _ := m.lookupUeByIMSI(victim.imsi); got != attacker {
		t.Fatal("after commit, the authenticated attach must supersede the prior context")
	}
}

// TestEstablishS1ConnectionMarksSecureExchange asserts the per-connection
// §4.4.4.3 flag is set when a UE resumes on a new connection — a resume only
// reaches establishS1Connection after its message was integrity-verified.
func TestEstablishS1ConnectionMarksSecureExchange(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	m.establishS1Connection(ue, &captureConn{}, 9)

	if !ue.s1.secureExchangeEstablished {
		t.Fatal("a verified resume must establish secure exchange on the new connection (TS 24.301 §4.4.4.3)")
	}
}

// TestVerifiedMessageMarksSecureExchange asserts a successfully integrity-checked
// message establishes secure exchange on a connection that did not have it yet
// (the fresh-attach case, where the flag is set when SMC Complete verifies).
func TestVerifiedMessageMarksSecureExchange(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.s1.secureExchangeEstablished = false // fresh connection, not yet established
	ue.ulCount = 0

	tac, err := (&eps.TrackingAreaUpdateComplete{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(tac, eps.SHTIntegrityProtectedCiphered, 0, nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(context.Background(), ue, wire)

	if !ue.s1.secureExchangeEstablished {
		t.Fatal("a verified message must establish secure exchange on the connection (TS 24.301 §4.4.4.3)")
	}
}
