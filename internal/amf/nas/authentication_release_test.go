// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func startTestAuthentication(t *testing.T, amfInstance *amf.AMF, fake *fakeAusf, ue *amf.UeContext, authCtxID string) {
	t.Helper()

	fake.AvKgAka = zeroChallenge(authCtxID)

	pass, err := authenticationProcedure(t.Context(), amfInstance, ue)
	if err != nil || pass {
		t.Fatalf("authenticationProcedure = %v, %v, want a challenge", pass, err)
	}

	if got := ue.Conn().AuthenticationCtx; got == nil || got.AuthCtxID != authCtxID {
		t.Fatalf("pending authentication context = %+v, want %q", got, authCtxID)
	}
}

func authenticatingTestUE(t *testing.T, authCtxID string) (*amf.AMF, *fakeAusf, *amf.UeContext) {
	t.Helper()

	amfInstance, fake := identityTestAMF(t, nil)
	ue, _ := registeredIdentityTestUE(t, amfInstance, amf.RegStepAuthenticating)
	ue.Tai = ue.Conn().Tai
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0})

	startTestAuthentication(t, amfInstance, fake, ue, authCtxID)

	return amfInstance, fake, ue
}

func assertFailureReported(t *testing.T, fake *fakeAusf, want ...string) {
	t.Helper()

	if got := fake.Failed(); !slices.Equal(got, want) {
		t.Fatalf("AUSF authentication failures = %q, want %q", got, want)
	}
}

func TestHappyPathConfirmsWithoutFailureReport(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-ok")
	conn := ue.Conn()

	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(make([]byte, 16)))

	if got := fake.Confirmed(); !slices.Equal(got, []string{"ctx-ok"}) {
		t.Fatalf("AUSF confirmed %q, want the pending context", got)
	}

	if ue.RegStep() != amf.RegStepSecurityMode {
		t.Fatalf("step = %v, want %v", ue.RegStep(), amf.RegStepSecurityMode)
	}

	conn.Release(t.Context())

	assertFailureReported(t, fake)
}

func TestSynchFailureReportsTheSupersededAuthentication(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-first")
	fake.AvKgAka = zeroChallenge("ctx-resync")

	auts := [14]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	handleAuthenticationFailure(t.Context(), amfInstance, ue, buildTestAuthenticationFailure(0x15, &auts))

	assertFailureReported(t, fake, "ctx-first")

	if got := ue.Conn().AuthenticationCtx.AuthCtxID; got != "ctx-resync" {
		t.Fatalf("pending authentication context = %q, want the re-synchronised challenge's", got)
	}

	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(make([]byte, 16)))

	if got := fake.Confirmed(); !slices.Equal(got, []string{"ctx-resync"}) {
		t.Fatalf("AUSF confirmed %q, want the re-synchronised context", got)
	}

	assertFailureReported(t, fake, "ctx-first")
}

func TestSecondSynchFailureReportsBothAuthentications(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-first")
	fake.AvKgAka = zeroChallenge("ctx-resync")

	auts := [14]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	handleAuthenticationFailure(t.Context(), amfInstance, ue, buildTestAuthenticationFailure(0x15, &auts))
	handleAuthenticationFailure(t.Context(), amfInstance, ue, buildTestAuthenticationFailure(0x15, &auts))

	assertFailureReported(t, fake, "ctx-first", "ctx-resync")
}

func TestAuthenticationFailureFromTheUEReportsTheAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause uint8
	}{
		{name: "MAC failure", cause: 0x14},
		{name: "non-5G authentication unacceptable", cause: 0x1a},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amfInstance, fake, ue := authenticatingTestUE(t, "ctx-failed")

			handleAuthenticationFailure(t.Context(), amfInstance, ue, buildTestAuthenticationFailure(tc.cause, nil))

			assertFailureReported(t, fake, "ctx-failed")

			if got := fake.Confirmed(); len(got) != 0 {
				t.Fatalf("AUSF confirmed %q, want none", got)
			}
		})
	}
}

func TestNgKSIAlreadyInUseKeepsTheAuthentication(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-kept")

	handleAuthenticationFailure(t.Context(), amfInstance, ue, buildTestAuthenticationFailure(0x47, nil))

	assertFailureReported(t, fake)

	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(make([]byte, 16)))

	if got := fake.Confirmed(); !slices.Equal(got, []string{"ctx-kept"}) {
		t.Fatalf("AUSF confirmed %q, want the kept context", got)
	}
}

func TestHRESStarMismatchReportsTheAuthentication(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-mismatch")

	res := make([]byte, 16)
	res[0] = 0xff
	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(res))

	assertFailureReported(t, fake, "ctx-mismatch")

	if got := fake.Confirmed(); len(got) != 0 {
		t.Fatalf("AUSF confirmed %q, want none", got)
	}
}

func TestMissingRESStarReportsTheAuthentication(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-no-res")

	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(nil))

	assertFailureReported(t, fake, "ctx-no-res")
}

func TestRestartedAuthenticationReportsTheSupersededAuthentication(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-first")

	startTestAuthentication(t, amfInstance, fake, ue, "ctx-second")

	assertFailureReported(t, fake, "ctx-first")
}

func TestT3560ExpiryReportsTheAuthentication(t *testing.T) {
	amfInstance, fake := identityTestAMF(t, nil)
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: time.Millisecond, MaxRetryTimes: 0}
	ue, _ := registeredIdentityTestUE(t, amfInstance, amf.RegStepAuthenticating)
	ue.Tai = ue.Conn().Tai

	startTestAuthentication(t, amfInstance, fake, ue, "ctx-expired")

	deadline := time.Now().Add(5 * time.Second)
	for len(fake.Failed()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	assertFailureReported(t, fake, "ctx-expired")
}

func TestConnectionTeardownReportsTheAuthentication(t *testing.T) {
	_, fake, ue := authenticatingTestUE(t, "ctx-teardown")
	conn := ue.Conn()

	if err := conn.AMFForTest().RemoveUeConn(t.Context(), conn); err != nil {
		t.Fatalf("RemoveUeConn: %v", err)
	}

	conn.Release(t.Context())

	assertFailureReported(t, fake, "ctx-teardown")
}

func TestDeregistrationReportsTheAuthentication(t *testing.T) {
	_, fake, ue := authenticatingTestUE(t, "ctx-deregistered")

	ue.Deregister(t.Context(), models.ReleaseFor(models.ReleaseRegistrationFailure))

	assertFailureReported(t, fake, "ctx-deregistered")
}

func TestAbortedRegistrationRetainingContextReportsTheAuthentication(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-suspended")

	amfInstance.AbortRegistrationRetainingContext(t.Context(), ue)

	assertFailureReported(t, fake, "ctx-suspended")
}

func TestChallengeArrivingAfterRegistrationMovedOnIsReported(t *testing.T) {
	amfInstance, fake, ue := authenticatingTestUE(t, "ctx-first")
	conn := ue.Conn()

	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(make([]byte, 16)))

	if ue.RegStep() != amf.RegStepSecurityMode {
		t.Fatalf("step = %v, want %v", ue.RegStep(), amf.RegStepSecurityMode)
	}

	fake.AvKgAka = zeroChallenge("ctx-late")

	requestChallenge(t.Context(), amfInstance, ue, conn, nil, func(_ context.Context, err error) {
		t.Fatalf("requestChallenge failed: %v", err)
	})

	assertFailureReported(t, fake, "ctx-late")

	if got := conn.AuthenticationCtx.AuthCtxID; got != "ctx-first" {
		t.Fatalf("pending authentication context = %q, want the confirmed one", got)
	}

	conn.Release(t.Context())

	assertFailureReported(t, fake, "ctx-late")
}
