// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func containerTestUE(t *testing.T, alg nas.CipheringAlgorithm, key [16]uint8) (*amf.AMF, *amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	supi := mustSUPIFromPrefixed("imsi-001019756139935")

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: "[\"000001\"]"},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))},
		Supi:    supi,
		Kseaf:   []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(supi)

	if err := amfInstance.CommitUEIdentity(t.Context(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.SetSecuredForTest(true)
	ue.SetKnasEncForTest(key)
	ue.SetCipheringAlgForTest(alg)

	return amfInstance, ue, ngapSender
}

func rejectCauses(t *testing.T, sender *fakeNGAPSender) []fgs.GMMCause {
	t.Helper()

	var causes []fgs.GMMCause

	for _, sent := range sender.SentDownlinkNASTransport {
		if len(sent.NASPDU) < 3 || sent.NASPDU[2] != uint8(fgs.MsgRegistrationReject) {
			continue
		}

		reject, err := fgs.ParseRegistrationReject(sent.NASPDU)
		if err != nil {
			t.Fatalf("ParseRegistrationReject: %v", err)
		}

		causes = append(causes, reject.Cause)
	}

	return causes
}

// TS 24.501 §5.4.2.2
func TestHandleRegistrationRequestMessage_UndecipherableContainerDoesNotReject(t *testing.T) {
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}

	m, err := buildTestRegistrationRequestMessage(nas.CipheringAES, &key, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	amfInstance, ue, ngapSender := containerTestUE(t, nas.CipheringZUC, key)

	_ = handleRegistrationRequestMessage(t.Context(), amfInstance, ue, regReqFgs(t, m), m, true, false)

	if causes := rejectCauses(t, ngapSender); len(causes) != 0 {
		t.Errorf("sent REGISTRATION REJECT %v; §5.4.2.2 requires the AMF to proceed on the cleartext IEs", causes)
	}

	if !ue.Conn().RetransmissionOfInitialNASMsg {
		t.Error("RINMR not set; the AMF must request retransmission of the initial NAS message")
	}
}

// TS 24.501 §5.5.1.2.8, §5.5.1.3.8
func TestHandleRegistrationRequestMessage_UndecodableContainerRejectsWithInvalidMandatoryInformation(t *testing.T) {
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}

	ciph, err := nas.CipherFor(nas.CipheringAES)
	if err != nil {
		t.Fatalf("CipherFor: %v", err)
	}

	garbage, err := ciph.Apply(key, 0, nas.Bearer3GPP, nas.DirectionUplink, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	outer := &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationTypeInitial,
		FOR:                  true,
		NgKSI:                nas.KeySetIdentifier{Value: 0},
		MobileIdentity:       testMobileIdentity(),
		UESecurityCapability: &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0},
		NASMessageContainer:  garbage,
	}

	m, err := outer.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	amfInstance, ue, ngapSender := containerTestUE(t, nas.CipheringAES, key)

	_ = handleRegistrationRequestMessage(t.Context(), amfInstance, ue, regReqFgs(t, m), m, true, false)

	causes := rejectCauses(t, ngapSender)
	if len(causes) != 1 {
		t.Fatalf("sent %d REGISTRATION REJECT messages (%v), want 1", len(causes), causes)
	}

	if causes[0] != fgs.GMMCauseInvalidMandatoryInformation {
		t.Errorf("cause = %v, want %v", causes[0], fgs.GMMCauseInvalidMandatoryInformation)
	}

	if causes[0] == fgs.GMMCauseUEIdentityCannotBeDerived {
		t.Error("#9 deregisters a UE on a mobility registration update (§5.5.1.3.5)")
	}
}
