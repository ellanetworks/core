// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

const identityTestSUPI = "imsi-001019756139935"

func assertAuthenticatedAs(t *testing.T, amfInstance *amf.AMF, want ...string) {
	t.Helper()

	fake, ok := amfInstance.Ausf.(*fakeAusf)
	if !ok {
		t.Fatalf("AUSF is %T, want *fakeAusf", amfInstance.Ausf)
	}

	if got := fake.Identities(); !slices.Equal(got, want) {
		t.Fatalf("AUSF authenticated %q, want %q", got, want)
	}
}

func zeroChallenge(authCtxID string) *ausf.UEAuthenticationCtx {
	zero := make([]byte, 16)
	hresStar := sha256.Sum256(append(append([]byte{}, zero...), zero...))

	return &ausf.UEAuthenticationCtx{
		AuthCtxID: authCtxID,
		Av5gAka: ausf.Av5gAka{
			Rand:      hex.EncodeToString(zero),
			HxresStar: hex.EncodeToString(hresStar[16:]),
			Autn:      hex.EncodeToString(zero),
		},
	}
}

func identityTestAMF(t *testing.T, challenge *ausf.UEAuthenticationCtx) (*amf.AMF, *fakeAusf) {
	t.Helper()

	fake := &fakeAusf{
		AvKgAka: challenge,
		Supi:    mustSUPIFromPrefixed(identityTestSUPI),
		Kseaf:   make([]byte, 32),
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
			Integrity:     `["AES","NULL"]`,
			Ciphering:     `["AES","NULL"]`,
		},
	}, fake, nil)
	amfInstance.NASGuardCfg.Enable = false

	return amfInstance, fake
}

func registeredIdentityTestUE(t *testing.T, amfInstance *amf.AMF, step amf.RegStep) (*amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.SetSuciForTest("suci-0-001-01-0000-0-0-9756139935")
	ue.SetSupiForTest(mustSUPIFromPrefixed(identityTestSUPI))

	if err := amfInstance.CommitUEIdentity(t.Context(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.ForceRegStepForTest(step)

	return ue, ngapSender
}

// TS 33.501 §6.1.2, TS 24.501 §5.5.1.2.8
func TestRestartedRegistrationByGUTIAuthenticatesTheSUPIAndCompletes5GAKA(t *testing.T) {
	amfInstance, fake := identityTestAMF(t, zeroChallenge("ctx-restarted"))
	ue, ngapSender := registeredIdentityTestUE(t, amfInstance, amf.RegStepSecurityMode)
	conn := ue.Conn()

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(t.Context(), amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	assertAuthenticatedAs(t, amfInstance, identityTestSUPI)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("sent %d downlinks, want the Authentication Request", len(ngapSender.SentDownlinkNASTransport))
	}

	assertPlainGmm(t, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgAuthenticationRequest))

	fresh := conn.UeContext()
	if fresh == nil || fresh == ue {
		t.Fatal("the registration did not restart on a fresh context")
	}

	if fresh.Supi().String() != identityTestSUPI {
		t.Fatalf("fresh context SUPI = %q, want %q", fresh.Supi(), identityTestSUPI)
	}

	handleAuthenticationResponse(t.Context(), amfInstance, fresh, buildAuthResponse(make([]byte, 16)))

	if got := fake.Confirmed(); !slices.Equal(got, []string{"ctx-restarted"}) {
		t.Fatalf("AUSF confirmed %q, want the context of the challenge", got)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 2 {
		t.Fatalf("sent %d downlinks, want the Security Mode Command after the Authentication Request", len(ngapSender.SentDownlinkNASTransport))
	}

	if sht := fgs.SecurityHeaderType(ngapSender.SentDownlinkNASTransport[1].NASPDU[1] & 0x0f); sht != fgs.SHTIntegrityProtectedNewContext {
		t.Fatalf("second downlink security header = %v, want the Security Mode Command's", sht)
	}

	if fresh.RegStep() != amf.RegStepSecurityMode {
		t.Fatalf("fresh context step = %v, want %v", fresh.RegStep(), amf.RegStepSecurityMode)
	}
}

// TS 33.501 §6.1.2, TS 24.501 §5.5.1.2.8
func TestRestartedRegistrationBySUCIAuthenticatesTheSUCI(t *testing.T) {
	amfInstance, _ := identityTestAMF(t, zeroChallenge("ctx-suci"))
	ue, ngapSender := registeredIdentityTestUE(t, amfInstance, amf.RegStepContextSetup)
	ue.Conn().RegistrationRequestPlain = []byte{0x7e, 0x00, 0x41}

	suci := fgs.SUCI{
		Format:           fgs.SUPIFormatIMSI,
		PLMN:             nas.PLMN{MCC: "001", MNC: "01"},
		RoutingIndicator: "0000",
		ProtectionScheme: fgs.ProtectionSchemeProfileA,
		SchemeOutput:     make([]byte, 45),
	}

	m, err := buildRegReqBytes(uint8(fgs.RegistrationTypeInitial), fgs.SUCIIdentity(suci), &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0}, 0, nil, 0, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(t.Context(), amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	assertAuthenticatedAs(t, amfInstance, suci.String())

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("sent %d downlinks, want the Authentication Request", len(ngapSender.SentDownlinkNASTransport))
	}

	assertPlainGmm(t, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgAuthenticationRequest))
}

// TS 33.501 §6.1.2, §6.1.3.3.2
func TestResynchronisationAuthenticatesTheRegistrationIdentityWithTheChallengeRAND(t *testing.T) {
	for _, tc := range []struct {
		name     string
		identity fgs.MobileIdentityType
		want     string
	}{
		{name: "registered by 5G-GUTI", identity: fgs.IdentityGUTI, want: identityTestSUPI},
		{name: "registered by SUCI", identity: fgs.IdentitySUCI, want: "suci-0-001-01-0000-0-0-9756139935"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amfInstance, fake := identityTestAMF(t, zeroChallenge("ctx-resync"))
			ue, ngapSender := registeredIdentityTestUE(t, amfInstance, amf.RegStepAuthenticating)
			ue.Tai = ue.Conn().Tai

			previous := &ausf.UEAuthenticationCtx{AuthCtxID: "ctx-first", Av5gAka: ausf.Av5gAka{Rand: "000102030405060708090a0b0c0d0e0f", Autn: hex.EncodeToString(make([]byte, 16))}}
			ue.Conn().AuthenticationCtx = previous
			ue.Conn().SetIdentityTypeUsedForRegistration(uint8(tc.identity))

			auts := [14]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
			handleAuthenticationFailure(t.Context(), amfInstance, ue, buildTestAuthenticationFailure(0x15, &auts))

			assertAuthenticatedAs(t, amfInstance, tc.want)

			resyncs := fake.Resyncs()
			if len(resyncs) != 1 || resyncs[0] == nil || resyncs[0].Rand != previous.Rand || resyncs[0].Auts != hex.EncodeToString(auts[:]) {
				t.Fatalf("AUSF re-synchronisation info = %+v, want the RAND of the failed challenge and the UE's AUTS", resyncs)
			}

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("sent %d downlinks, want the new Authentication Request", len(ngapSender.SentDownlinkNASTransport))
			}

			assertPlainGmm(t, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgAuthenticationRequest))

			if ue.Conn().AuthenticationCtx.AuthCtxID != "ctx-resync" {
				t.Fatalf("pending authentication context = %q, want the re-synchronised challenge's", ue.Conn().AuthenticationCtx.AuthCtxID)
			}
		})
	}
}
