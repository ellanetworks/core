package gmm

import (
	"fmt"
	"testing"
	"time"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

func newTestAMF() *amfContext.AMF {
	amf := amfContext.New(&FakeDBInstance{
		Operator: &db.Operator{
			SpnFullName:  "Ella Networks",
			SpnShortName: "Ella",
		},
	}, nil, nil)

	return amf
}

func setupRegistrationCompleteUE(t *testing.T) (*amfContext.AmfUe, *FakeNGAPSender) {
	t.Helper()

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = mustSUPIFromPrefixed("imsi-001019756139935")
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount.Get())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.State = amfContext.ContextSetup
	ue.T3550 = amfContext.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.RegistrationRequest = m.RegistrationRequest
	ue.RegistrationType5GS = 42
	ue.IdentityTypeUsedForRegistration = 42
	ue.AuthFailureCauseSynchFailureTimes = 42
	ue.RanUe.UeContextRequest = true
	ue.RanUe.RecvdInitialContextSetupResponse = true
	ue.RetransmissionOfInitialNASMsg = true
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)

	return ue, ngapSender
}

func TestHandleRegistrationComplete_WrongState_Error(t *testing.T) {
	testcases := []amfContext.StateType{amfContext.Deregistered, amfContext.Authentication, amfContext.Registered, amfContext.SecurityMode}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := amfContext.NewAmfUe()
			ue.State = tc

			expected := fmt.Sprintf("state mismatch: receive Registration Complete message in state %s", tc)

			err := handleRegistrationComplete(t.Context(), newTestAMF(), ue)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got: %v", expected, err)
			}
		})
	}
}

func TestHandleRegistrationComplete_T3550StoppedAndCleared_RegistrationDataCleared(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)

	amf := newTestAMF()

	err := handleRegistrationComplete(t.Context(), amf, ue)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	if ue.T3550 != nil {
		t.Fatalf("expected timer T3550 to be stopped and cleared")
	}

	err = checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_SendsConfigurationUpdateCommand(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)

	amf := newTestAMF()

	err := handleRegistrationComplete(t.Context(), amf, ue)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

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

	err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, 0, security.Bearer3GPP, security.DirectionDownlink, payload)
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
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.RegistrationRequest.UplinkDataStatus = nil
	ue.SmContextList = make(map[uint8]*amfContext.SmContext)

	amf := newTestAMF()

	err := handleRegistrationComplete(t.Context(), amf, ue)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("should have sent a UE Context Release Command message")
	}

	if ue.T3550 != nil {
		t.Fatalf("expected timer T3550 to be stopped and cleared")
	}

	err = checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_NotReleasedWhenFORPending(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestPending)
	ue.RegistrationRequest.UplinkDataStatus = nil
	ue.SmContextList = make(map[uint8]*amfContext.SmContext)

	amf := newTestAMF()

	err := handleRegistrationComplete(t.Context(), amf, ue)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	if ue.T3550 != nil {
		t.Fatalf("expected timer T3550 to be stopped and cleared")
	}

	err = checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_NotReleasedWhenUDSPending(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{}
	ue.SmContextList = make(map[uint8]*amfContext.SmContext)

	amf := newTestAMF()

	err := handleRegistrationComplete(t.Context(), amf, ue)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	if ue.T3550 != nil {
		t.Fatalf("expected timer T3550 to be stopped and cleared")
	}

	err = checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func TestHandleRegistrationComplete_NotReleasedWhenActiveSession(t *testing.T) {
	ue, ngapSender := setupRegistrationCompleteUE(t)
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.RegistrationRequest.UplinkDataStatus = nil
	_ = ue.CreateSmContext(1, "testref1", &models.Snssai{})

	amf := newTestAMF()

	err := handleRegistrationComplete(t.Context(), amf, ue)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("should not have sent a UE Context Release Command message")
	}

	if ue.T3550 != nil {
		t.Fatalf("expected timer T3550 to be stopped and cleared")
	}

	err = checkUERegistrationDataIsCleared(ue)
	if err != nil {
		t.Fatalf("expected ue registration data to be cleared: %v", err)
	}
}

func checkUERegistrationDataIsCleared(ue *amfContext.AmfUe) error {
	if ue.RegistrationRequest != nil {
		return fmt.Errorf("registration request is not nil")
	}

	if ue.RegistrationType5GS != 0 {
		return fmt.Errorf("registration type 5gs was not 0: %d", ue.RegistrationType5GS)
	}

	if ue.IdentityTypeUsedForRegistration != 0 {
		return fmt.Errorf("identity type used for registration was not 0: %d", ue.IdentityTypeUsedForRegistration)
	}

	if ue.AuthFailureCauseSynchFailureTimes != 0 {
		return fmt.Errorf("auth failure caush synch failure times was not 0: %d", ue.AuthFailureCauseSynchFailureTimes)
	}

	if ue.RanUe.UeContextRequest {
		return fmt.Errorf("ranue context request should be false")
	}

	if ue.RanUe.RecvdInitialContextSetupResponse {
		return fmt.Errorf("ranue recvd initial context setup response should be false")
	}

	if ue.RetransmissionOfInitialNASMsg {
		return fmt.Errorf("retransmission of initial NAS msg should be false")
	}

	if ue.GetOnGoing() != amfContext.OnGoingProcedureNothing {
		return fmt.Errorf("ongoing should be nothing")
	}

	return nil
}
