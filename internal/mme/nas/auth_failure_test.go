// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
)

// authChallengedUE returns a UE that has been sent an Authentication Request
// (auth vector with a fixed RAND), as it would be mid-authentication.
func authChallengedUE(t *testing.T, m *mme.MME) (*mme.UeContext, *captureConn) {
	t.Helper()

	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)

	var r [16]byte
	for i := range r {
		r[i] = byte(i + 1)
	}

	ue.AuthVector = &udm.EPSAV{RAND: r}

	return ue, cc
}

// autsFor builds a valid AUTS for sqnMS using the hard-coded subscriber's
// credentials and the UE's challenge RAND, as a UE would on a synch failure.
func autsFor(t *testing.T, ue *mme.UeContext, sqnMS []byte) []byte {
	t.Helper()

	opc, k, rand := testSubscriber.OPc[:], testSubscriber.K[:], ue.AuthVector.RAND[:]

	ak := make([]byte, 6)
	if err := udm.F2345(opc, k, rand, nil, nil, nil, nil, ak); err != nil {
		t.Fatal(err)
	}

	conc := make([]byte, 6)
	for i := range conc {
		conc[i] = sqnMS[i] ^ ak[i]
	}

	macS := make([]byte, 8)
	if err := udm.F1(opc, k, rand, sqnMS, []byte{0x00, 0x00}, nil, macS); err != nil {
		t.Fatal(err)
	}

	return append(conc, macS...)
}

func authFailure(t *testing.T, cause uint8, auts []byte) []byte {
	t.Helper()

	b, err := (&eps.AuthenticationFailure{Cause: cause, AUTS: auts}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// TestAuthenticationResponseWrongRESRejects checks that a UE answering the
// challenge with a RES that does not match the expected XRES is refused with
// AUTHENTICATION REJECT and its S1 context released (TS 24.301 §5.4.2.5),
// matching the AUTHENTICATION FAILURE path, and gains no security context.
func TestAuthenticationResponseWrongRESRejects(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	resp, err := (&eps.AuthenticationResponse{RES: []byte{1, 2, 3, 4, 5, 6, 7, 8}}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(m, context.Background(), ue, resp)

	// Authentication Reject (downlink NAS) followed by UE Context Release Command.
	if cc.count() != 2 {
		t.Fatalf("expected Auth Reject + Release Command, got %d", cc.count())
	}

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("not an Authentication Reject: %v", err)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])

	if len(ue.KASMEForTest()) != 0 || ue.Secured() {
		t.Fatal("UE gained a security context despite a RES mismatch")
	}
}

func TestAuthFailureMACFailureRejects(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	HandleNAS(m, context.Background(), ue, authFailure(t, mme.EmmCauseMACFailure, nil))

	// Authentication Reject (downlink NAS) followed by UE Context Release Command.
	if len(cc.sent) != 2 {
		t.Fatalf("expected Auth Reject + Release Command, got %d", len(cc.sent))
	}

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("not an Authentication Reject: %v", err)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

func TestAuthFailureSynchResyncsAndReauthenticates(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	auts := autsFor(t, ue, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x21})

	HandleNAS(m, context.Background(), ue, authFailure(t, mme.EmmCauseSynchFailure, auts))

	// A fresh Authentication Request, not a reject.
	if len(cc.sent) != 1 {
		t.Fatalf("expected a re-sent Authentication Request, got %d messages", len(cc.sent))
	}

	if _, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("not an Authentication Request: %v", err)
	}

	if !ue.ResyncTried() {
		t.Fatal("resyncTried not set")
	}

	// A second synch failure must not resync again — it rejects.
	HandleNAS(m, context.Background(), ue, authFailure(t, mme.EmmCauseSynchFailure, auts))

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[1])); err != nil {
		t.Fatalf("second synch failure not rejected: %v", err)
	}
}

func TestAuthFailureBadAUTSRejects(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	auts := autsFor(t, ue, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x21})
	auts[len(auts)-1] ^= 0xff // corrupt MAC-S

	HandleNAS(m, context.Background(), ue, authFailure(t, mme.EmmCauseSynchFailure, auts))

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("bad AUTS not rejected: %v", err)
	}
}
