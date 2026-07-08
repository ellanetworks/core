// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

func newTestAMF() *amf.AMF {
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			SpnFullName:  "Ella Networks",
			SpnShortName: "Ella",
		},
	}, nil, nil)

	return amfInstance
}

func setupRegistrationCompleteUE(t *testing.T) (*amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCountForTest().Value())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepContextSetup)
	ue.Conn().RegistrationRequest = m.RegistrationRequest
	ue.Conn().RegistrationType5GS = 42
	ue.Conn().IdentityTypeUsedForRegistration = 42
	ue.Conn().SetResyncTried(true)
	ue.Conn().UeContextRequest = true
	ue.Conn().MarkICSCompleted()
	ue.Conn().RetransmissionOfInitialNASMsg = true

	return ue, ngapSender
}

func TestHandleRegistrationComplete_WrongState_Ignored(t *testing.T) {
	testcases := []struct {
		name  string
		setup func(*amf.UeContext)
		state amf.StateType
	}{
		{"Deregistered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Deregistered) }, amf.Deregistered},
		{"Registered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Registered) }, amf.Registered},
		{"Authenticating", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepAuthenticating) }, amf.RegistrationInitiated},
		{"SecurityMode", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepSecurityMode) }, amf.RegistrationInitiated},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue := amf.NewUeContext()
			tc.setup(ue)

			handleRegistrationComplete(t.Context(), newTestAMF(), ue)

			if ue.State() != tc.state {
				t.Fatalf("expected out-of-context-setup Registration Complete to leave state %s unchanged, got %s", tc.state, ue.State())
			}
		})
	}
}

func TestHandleRegistrationComplete_T3550StoppedAndCleared_RegistrationDataCleared(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)

	amfInstance := newTestAMF()

	handleRegistrationComplete(t.Context(), amfInstance, ue)

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	err := checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_SendsConfigurationUpdateCommand(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)

	amfInstance := newTestAMF()

	handleRegistrationComplete(t.Context(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport (ConfigurationUpdateCommand), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nasPdu := ngapSender.SentDownlinkNASTransport[0].NasPdu

	// Decrypt the NAS message: strip EPD(1) + SecHeader(1) + MAC(4) + SQN(1) = 7 bytes
	if len(nasPdu) < 7 {
		t.Fatalf("NAS PDU too short: %d bytes", len(nasPdu))
	}

	payload := make([]byte, len(nasPdu)-7)
	copy(payload, nasPdu[7:])

	err := security.NASEncrypt(ue.CipheringAlgForTest(), ue.KnasEncForTest(), 0, security.Bearer3GPP, security.DirectionDownlink, payload)
	if err != nil {
		t.Fatalf("NAS decrypt failed: %v", err)
	}

	msg := new(nas.Message)

	err = msg.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("NAS decode failed: %v", err)
	}

	cuc := msg.ConfigurationUpdateCommand
	if cuc == nil {
		t.Fatal("expected ConfigurationUpdateCommand message")
	}

	// Registration complete sends NITZ only (no GUTI reassignment)
	if cuc.GUTI5G != nil {
		t.Fatal("expected no GUTI in ConfigurationUpdateCommand after registration complete")
	}

	if cuc.FullNameForNetwork == nil {
		t.Fatal("expected FullNameForNetwork in ConfigurationUpdateCommand")
	}

	if cuc.ShortNameForNetwork == nil {
		t.Fatal("expected ShortNameForNetwork in ConfigurationUpdateCommand")
	}
}

func TestHandleRegistrationComplete_ReleasedWhenNoFORPending_NoUDSPending_and_NoActiveSessions(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)
	ue.Conn().RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.Conn().RegistrationRequest.UplinkDataStatus = nil
	ue.SmContextList = make(map[uint8]*amf.SmContext)

	amfInstance := newTestAMF()

	handleRegistrationComplete(t.Context(), amfInstance, ue)

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("should have sent a UE Context Release Command message")
	}

	err := checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_NotReleasedWhenFORPending(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)
	ue.Conn().RegistrationRequest.SetFOR(nasMessage.FollowOnRequestPending)
	ue.Conn().RegistrationRequest.UplinkDataStatus = nil
	ue.SmContextList = make(map[uint8]*amf.SmContext)

	amfInstance := newTestAMF()

	handleRegistrationComplete(t.Context(), amfInstance, ue)

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	err := checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_NotReleasedWhenUDSPending(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)
	ue.Conn().RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.Conn().RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{}
	ue.SmContextList = make(map[uint8]*amf.SmContext)

	amfInstance := newTestAMF()

	handleRegistrationComplete(t.Context(), amfInstance, ue)

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	err := checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_NotReleasedWhenActiveSession(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)
	ue.Conn().RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.Conn().RegistrationRequest.UplinkDataStatus = nil
	_ = ue.CreateSmContext(1, "testref1", &models.Snssai{})

	amfInstance := newTestAMF()

	handleRegistrationComplete(t.Context(), amfInstance, ue)

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	err := checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func checkUERegistrationDataIsCleared(ue *amf.UeContext) error {
	if ue.Conn().RegistrationRequest != nil {
		return fmt.Errorf("registration request is not nil")
	}

	if ue.Conn().RegistrationType5GS != 0 {
		return fmt.Errorf("registration type 5gs was not 0: %d", ue.Conn().RegistrationType5GS)
	}

	if ue.Conn().IdentityTypeUsedForRegistration != 0 {
		return fmt.Errorf("identity type used for registration was not 0: %d", ue.Conn().IdentityTypeUsedForRegistration)
	}

	if ue.Conn().ResyncTried() {
		return fmt.Errorf("resyncTried was not reset on registration complete")
	}

	if ue.Conn().UeContextRequest {
		return fmt.Errorf("ranue context request should be false")
	}

	if ue.Conn().RetransmissionOfInitialNASMsg {
		return fmt.Errorf("retransmission of initial NAS msg should be false")
	}

	if ue.PagingActive() {
		return fmt.Errorf("ongoing should be nothing")
	}

	return nil
}
