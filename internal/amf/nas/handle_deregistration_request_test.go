// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// TestHandleDeregistrationRequest_ProcessedInAnyState verifies a UE-initiated
// Deregistration Request is processed regardless of the UE's state — TS 24.501
// §5.5.2.2.2 (like TS 24.301 §5.5.2.2.2) has no state precondition; the integrity
// guard is the security control. Mirrors the MME's state-unguarded detach handling.
func TestHandleDeregistrationRequest_ProcessedInAnyState(t *testing.T) {
	testcases := []amf.StateType{amf.Deregistered, amf.RegistrationInitiated, amf.DeregistrationInitiated, amf.Registered}
	for _, tc := range testcases {
		t.Run(fmt.Sprintf("State-%s", tc), func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build test ue: %v", err)
			}

			ue.ForceStateForTest(tc)

			m := buildTestDeregistrationRequestUEOriginatingDeregistration()

			handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), ue, m, true)

			if len(ngapSender.SentUEContextReleaseCommand) != 1 {
				t.Fatalf("expected a UE Context Release Command in state %s, got %d", tc, len(ngapSender.SentUEContextReleaseCommand))
			}
		})
	}
}

func TestHandleRegistrationRequest_AllSmContextAreReleased(t *testing.T) {
	smf := fakeSmf{Error: nil, ReleasedSmContext: make([]string, 0)}
	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, &smf)

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	ue.ForceStateForTest(amf.Registered)
	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("could not add UE to amf.AMF pool: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	_ = ue.CreateSmContext(1, "testref1", &snssai, "internet")
	_ = ue.CreateSmContext(2, "testref2", &snssai, "internet")
	_ = ue.CreateSmContext(3, "testref3", &snssai, "internet")
	_ = ue.CreateSmContext(4, "testref4", &snssai, "internet")

	m := buildTestDeregistrationRequestUEOriginatingDeregistration()

	handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), ue, m, true)

	r := smf.ReleasedSmContext

	if len(r) != 4 {
		t.Fatalf("expected 4 amf.SmContext to be relased, released: %d", len(r))
	}

	if !slices.Contains(r, "testref1") || !slices.Contains(r, "testref2") || !slices.Contains(r, "testref3") || !slices.Contains(r, "testref4") {
		t.Fatalf("expected all SM Contexts to be release, released: %v", r)
	}
}

func TestHandleDeregistrationRequest_NilRanUE(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.Conn().AMFForTest().ReleaseNasConnection(ue, nil)

	m := buildTestDeregistrationRequestUEOriginatingDeregistration()

	handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), ue, m, true)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatal("should not have sent a downlink NAS transport message")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatal("should not have sent a downlink NAS transport message")
	}
}

func TestHandleDeregistrationRequest_NotSwitchOff_DeregistrationAccept(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	m := buildTestDeregistrationRequestUEOriginatingDeregistration()

	handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), ue, m, true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatal("should have sent a downlink NAS transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDeregistrationAcceptUEOrig))

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatal("should have sent a UE Context Release Command message")
	}
}

func TestHandleDeregistrationRequest_SwitchOff_NoDeregistrationAccept(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	m := buildDeregRequestUEOrig(fgs.AccessType3GPP, true)

	handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), ue, m, true)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatal("should have sent a downlink NAS transport message")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatal("should have sent a UE Context Release Command message")
	}
}

// TestHandleDeregistrationRequest_MacFailed_RejectsForgery verifies the
// handler rejects a MacFailed Deregistration Request while the amf.AMF still
// holds a valid security context (TS 24.501 defense in depth).
func TestHandleDeregistrationRequest_MacFailed_RejectsForgery(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(true)

	m := buildTestDeregistrationRequestUEOriginatingDeregistration()

	handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), ue, m, false)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatal("must not send Deregistration Accept on a forged request")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatal("must not release UE context on a forged request")
	}

	if ue.State() != amf.Registered {
		t.Fatalf("UE must remain amf.Registered after rejecting forgery, got %s", ue.State())
	}

	if !ue.SecuredForTest() {
		t.Error("handler must not tear down SecurityContextAvailable on a forged request")
	}
}

func TestHandleDeregistrationRequest_Non3GPP_DeregistrationAccept(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	m := buildDeregRequestUEOrig(fgs.AccessTypeNon3GPP, false)

	handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), ue, m, true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatal("should have sent a downlink NAS transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDeregistrationAcceptUEOrig))

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatal("should not have sent a UE Context Release Command message")
	}
}

func buildTestDeregistrationRequestUEOriginatingDeregistration() *fgs.DeregistrationRequestUEOriginating {
	return buildDeregRequestUEOrig(fgs.AccessType3GPP, false)
}

// buildDeregRequestUEOrig builds a UE-originating DEREGISTRATION REQUEST with the
// given de-registration type (TS 24.501 §8.2.12, §9.11.3.20).
func buildDeregRequestUEOrig(accessType fgs.AccessType, switchOff bool) *fgs.DeregistrationRequestUEOriginating {
	return &fgs.DeregistrationRequestUEOriginating{
		AccessType: accessType,
		SwitchOff:  switchOff,
		MobileIdentity: fgs.GUTIIdentity(fgs.GUTI{
			PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 0, AMFPointer: 0,
			TMSI: [4]byte{0x00, 0x00, 0x00, 0x01},
		}),
	}
}
