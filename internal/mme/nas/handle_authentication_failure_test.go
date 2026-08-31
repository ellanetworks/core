// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §4.4.4.3
func TestAuthenticationFailureIgnoredWithNoAuthInProgress(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	plain := &eps.AuthenticationFailure{Cause: eps.EMMCauseMACFailure}

	handleAuthenticationFailure(context.Background(), m, ue, ue.Conn(), plain)

	if ue.Conn() == nil || ue.ReleasingForTest() {
		t.Fatal("a spurious Authentication Failure must not release the UE")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP message for an ignored failure, got %d", cc.count())
	}
}

// TS 24.301 §7.4
func TestAuthenticationFailureDuringSecurityModeIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	ue.ForceRegStepForTest(mme.RegStepSecurityMode)

	handleAuthenticationFailure(context.Background(), m, ue, ue.Conn(), authFailure(t, eps.EMMCauseMACFailure, nil))

	if ue.Conn() == nil || ue.ReleasingForTest() {
		t.Fatal("an out-of-phase Authentication Failure must not release the UE")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP message for an ignored failure, got %d", cc.count())
	}
}

// TS 24.301 §5.4.2.7
func TestFreshAuthenticationResetsResyncBudget(t *testing.T) {
	m := newTestMME(t)
	ue, _ := authChallengedUE(t, m)

	ue.Conn().SetResyncTried(true)

	startAuthentication(context.Background(), m, ue, ue.Conn(), models.PlmnID{Mcc: "001", Mnc: "01"})

	if ue.Conn().ResyncTried() {
		t.Fatal("startAuthentication must reset resyncTried for a fresh authentication")
	}
}

func authChallengedUE(t *testing.T, m *mme.MME) (*mme.UeContext, *captureConn) {
	t.Helper()

	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.ForceRegStepForTest(mme.RegStepAuthenticating)

	var r [16]byte
	for i := range r {
		r[i] = byte(i + 1)
	}

	ue.Conn().AuthVector = &udm.EPSAV{RAND: r}

	return ue, cc
}

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

func authFailure(_ *testing.T, cause eps.EMMCause, auts []byte) *eps.AuthenticationFailure {
	return &eps.AuthenticationFailure{Cause: cause, AUTS: auts}
}

func authFailureWire(t *testing.T, cause eps.EMMCause, auts []byte) []byte {
	t.Helper()

	b, err := authFailure(t, cause, auts).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// TS 24.301 §5.4.2.5
func TestAuthenticationResponseWrongRESRejects(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)

	resp, err := (&eps.AuthenticationResponse{RES: []byte{1, 2, 3, 4, 5, 6, 7, 8}}).MarshalBinary()
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

	HandleNAS(context.Background(), m, ue.Conn(), authFailureWire(t, eps.EMMCauseMACFailure, nil))

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

	HandleNAS(context.Background(), m, ue.Conn(), authFailureWire(t, eps.EMMCauseSynchFailure, auts))

	if len(cc.sent) != 1 {
		t.Fatalf("expected a re-sent Authentication Request, got %d messages", len(cc.sent))
	}

	if _, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("not an Authentication Request: %v", err)
	}

	if !ue.Conn().ResyncTried() {
		t.Fatal("resyncTried not set")
	}

	HandleNAS(context.Background(), m, ue.Conn(), authFailureWire(t, eps.EMMCauseSynchFailure, auts))

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[1])); err != nil {
		t.Fatalf("second synch failure not rejected: %v", err)
	}
}

// TS 24.301 §7.8
func TestAuthFailureOutOfEnumerationCauseIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, cc := authChallengedUE(t, m)
	ue.Conn().ArmNASGuard("Authentication Request", []byte{0x07, 0x52}, eps.SHTPlain)

	handleAuthenticationFailure(context.Background(), m, ue, ue.Conn(), authFailure(t, eps.EMMCauseProtocolErrorUnspecified, nil))

	if ue.Conn() == nil || ue.ReleasingForTest() {
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
	auts[len(auts)-1] ^= 0xff

	HandleNAS(context.Background(), m, ue.Conn(), authFailureWire(t, eps.EMMCauseSynchFailure, auts))

	if _, err := eps.ParseAuthenticationReject(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("bad AUTS not rejected: %v", err)
	}
}
