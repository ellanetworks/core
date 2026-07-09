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

// TestAuthenticationFailureIgnoredWithNoAuthInProgress verifies a spurious
// AUTHENTICATION FAILURE (no authentication in flight) does not release the UE —
// the message is admissible without integrity (TS 24.301 §4.4.4.3), so an
// out-of-order one must not tear down a UE. Mirrors the AMF's RegStep gating.
func TestAuthenticationFailureIgnoredWithNoAuthInProgress(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	// No AuthVector: no authentication is in progress.
	plain, err := (&eps.AuthenticationFailure{Cause: mme.EmmCauseMACFailure}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleAuthenticationFailure(context.Background(), m, ue, plain)

	if ue.Conn() == nil || ue.Conn().ReleasingForTest() {
		t.Fatal("a spurious Authentication Failure must not release the UE")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP message for an ignored failure, got %d", cc.count())
	}
}

// TestAuthenticationFailureDuringSecurityModeIgnored proves the RegStep gate drops an
// out-of-phase AUTHENTICATION FAILURE on its own. A real UE clears its auth vector on
// authentication success, so this forces a vector set alongside RegStepSecurityMode to
// isolate the RegStep gate — not the AuthVector==nil gate — as what drops an unprotected
// #20 injected during the security-mode sub-phase (out-of-state handling is
// implementation-dependent, TS 24.301 §7.4).
func TestAuthenticationFailureDuringSecurityModeIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	// Force the security-mode sub-phase with a vector still set (a real UE clears it on
	// auth success) so the RegStep gate is the only thing that can drop the failure.
	ue.ForceRegStepForTest(mme.RegStepSecurityMode)

	handleAuthenticationFailure(context.Background(), m, ue, authFailure(t, mme.EmmCauseMACFailure, nil))

	if ue.Conn() == nil || ue.Conn().ReleasingForTest() {
		t.Fatal("an out-of-phase Authentication Failure must not release the UE")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP message for an ignored failure, got %d", cc.count())
	}
}

// TestFreshAuthenticationResetsResyncBudget verifies a new authentication procedure
// resets resyncTried, so a genuine synch failure on a later authentication (on a reused
// persistent UE context) is not wrongly refused a resync. resyncTried scopes to one
// exchange's consecutive synch failures (TS 24.301 §5.4.2.7); the AMF likewise resets
// its per-connection synch-failure counter.
func TestFreshAuthenticationResetsResyncBudget(t *testing.T) {
	m := newTestMME(t)
	ue, _ := authChallengedUE(t, m)

	// A prior authentication exchange already spent its resync.
	ue.Conn().SetResyncTried(true)

	startAuthentication(context.Background(), m, ue)

	if ue.Conn().ResyncTried() {
		t.Fatal("startAuthentication must reset resyncTried for a fresh authentication")
	}
}

// authChallengedUE returns a UE that has been sent an Authentication Request
// (auth vector with a fixed RAND), as it would be mid-authentication.
func authChallengedUE(t *testing.T, m *mme.MME) (*mme.UeContext, *captureConn) {
	t.Helper()

	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.ForceRegStepForTest(mme.RegStepAuthenticating)

	var r [16]byte
	for i := range r {
		r[i] = byte(i + 1)
	}

	ue.Conn().AuthVector = &udm.EPSAV{RAND: r}

	return ue, cc
}

// autsFor builds a valid AUTS for sqnMS using the hard-coded subscriber's
// credentials and the UE's challenge RAND, as a UE would on a synch failure.
func autsFor(t *testing.T, ue *mme.UeContext, sqnMS []byte) []byte {
	t.Helper()

	opc, k, rand := testSubscriber.OPc[:], testSubscriber.K[:], ue.Conn().AuthVector.RAND[:]

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

	HandleNAS(context.Background(), m, ue.Conn(), resp)

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

	HandleNAS(context.Background(), m, ue.Conn(), authFailure(t, mme.EmmCauseMACFailure, nil))

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

	HandleNAS(context.Background(), m, ue.Conn(), authFailure(t, mme.EmmCauseSynchFailure, auts))

	// A fresh Authentication Request, not a reject.
	if len(cc.sent) != 1 {
		t.Fatalf("expected a re-sent Authentication Request, got %d messages", len(cc.sent))
	}

	if _, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("not an Authentication Request: %v", err)
	}

	if !ue.Conn().ResyncTried() {
		t.Fatal("resyncTried not set")
	}

	// A second synch failure must not resync again — it rejects.
	HandleNAS(context.Background(), m, ue.Conn(), authFailure(t, mme.EmmCauseSynchFailure, auts))

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[1])); err != nil {
		t.Fatalf("second synch failure not rejected: %v", err)
	}
}

// TestAuthFailureOutOfEnumerationCauseIgnored verifies an AUTHENTICATION FAILURE
// carrying a cause outside the enumeration (#20, #21, #26) is ignored — the UE is not
// released and the guard is left armed — rather than teardown on a semantically
// incorrect message (TS 24.301 §7.8). Mirrors the AMF.
func TestAuthFailureOutOfEnumerationCauseIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)
	ue.Conn().ArmNASGuard("Authentication Request", []byte{0x07, 0x52})

	// #111 "protocol error, unspecified" is a valid EMM cause but not an
	// AUTHENTICATION FAILURE cause.
	handleAuthenticationFailure(context.Background(), m, ue, authFailure(t, mme.EmmCauseProtocolErrorUnspec, nil))

	if ue.Conn() == nil || ue.Conn().ReleasingForTest() {
		t.Fatal("an out-of-enumeration Authentication Failure cause must not release the UE")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP message for an ignored failure, got %d", cc.count())
	}

	if !ue.NASGuardActiveForTest() {
		t.Fatal("the authentication guard must stay armed on an out-of-enumeration cause")
	}
}

func TestAuthFailureBadAUTSRejects(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	auts := autsFor(t, ue, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x21})
	auts[len(auts)-1] ^= 0xff // corrupt MAC-S

	HandleNAS(context.Background(), m, ue.Conn(), authFailure(t, mme.EmmCauseSynchFailure, auts))

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("bad AUTS not rejected: %v", err)
	}
}
