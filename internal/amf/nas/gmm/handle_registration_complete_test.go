package gmm

import (
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

func TestHandleRegistrationComplete_WrongState_Error(t *testing.T) {
	testcases := []context.StateType{context.Deregistered, context.Authentication, context.Registered, context.SecurityMode}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := context.NewAmfUe()
			ue.State = tc

			expected := fmt.Sprintf("state mismatch: receive Registration Complete message in state %s", tc)

			err := handleRegistrationComplete(t.Context(), ue)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got: %v", expected, err)
			}
		})
	}
}

func TestHandleRegistrationComplete_T3550StoppedAndCleared_RegistrationDataCleared(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

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

	ue.State = context.ContextSetup
	ue.T3550 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.RegistrationRequest = m.RegistrationRequest
	ue.RegistrationType5GS = 42
	ue.IdentityTypeUsedForRegistration = 42
	ue.AuthFailureCauseSynchFailureTimes = 42
	ue.RanUe.UeContextRequest = true
	ue.RanUe.RecvdInitialContextSetupResponse = true
	ue.RetransmissionOfInitialNASMsg = true
	ue.SetOnGoing(context.OnGoingProcedurePaging)

	err = handleRegistrationComplete(t.Context(), ue)
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

func TestHandleRegistrationComplete_ReleasedWhenNoFORPending_NoUDSPending_and_NoActiveSessions(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

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

	ue.State = context.ContextSetup
	ue.T3550 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.RegistrationRequest = m.RegistrationRequest
	ue.RegistrationType5GS = 42
	ue.IdentityTypeUsedForRegistration = 42
	ue.AuthFailureCauseSynchFailureTimes = 42
	ue.RanUe.UeContextRequest = true
	ue.RanUe.RecvdInitialContextSetupResponse = true
	ue.RetransmissionOfInitialNASMsg = true
	ue.SetOnGoing(context.OnGoingProcedurePaging)
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.RegistrationRequest.UplinkDataStatus = nil
	ue.SmContextList = make(map[uint8]*context.SmContext)

	err = handleRegistrationComplete(t.Context(), ue)
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
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

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

	ue.State = context.ContextSetup
	ue.T3550 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.RegistrationRequest = m.RegistrationRequest
	ue.RegistrationType5GS = 42
	ue.IdentityTypeUsedForRegistration = 42
	ue.AuthFailureCauseSynchFailureTimes = 42
	ue.RanUe.UeContextRequest = true
	ue.RanUe.RecvdInitialContextSetupResponse = true
	ue.RetransmissionOfInitialNASMsg = true
	ue.SetOnGoing(context.OnGoingProcedurePaging)
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestPending)
	ue.RegistrationRequest.UplinkDataStatus = nil
	ue.SmContextList = make(map[uint8]*context.SmContext)

	err = handleRegistrationComplete(t.Context(), ue)
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
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

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

	ue.State = context.ContextSetup
	ue.T3550 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.RegistrationRequest = m.RegistrationRequest
	ue.RegistrationType5GS = 42
	ue.IdentityTypeUsedForRegistration = 42
	ue.AuthFailureCauseSynchFailureTimes = 42
	ue.RanUe.UeContextRequest = true
	ue.RanUe.RecvdInitialContextSetupResponse = true
	ue.RetransmissionOfInitialNASMsg = true
	ue.SetOnGoing(context.OnGoingProcedurePaging)
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{}
	ue.SmContextList = make(map[uint8]*context.SmContext)

	err = handleRegistrationComplete(t.Context(), ue)
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
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

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

	ue.State = context.ContextSetup
	ue.T3550 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.RegistrationRequest = m.RegistrationRequest
	ue.RegistrationType5GS = 42
	ue.IdentityTypeUsedForRegistration = 42
	ue.AuthFailureCauseSynchFailureTimes = 42
	ue.RanUe.UeContextRequest = true
	ue.RanUe.RecvdInitialContextSetupResponse = true
	ue.RetransmissionOfInitialNASMsg = true
	ue.SetOnGoing(context.OnGoingProcedurePaging)
	ue.RegistrationRequest.SetFOR(nasMessage.FollowOnRequestNoPending)
	ue.RegistrationRequest.UplinkDataStatus = nil
	ue.CreateSmContext(1, "testref1", &models.Snssai{})

	err = handleRegistrationComplete(t.Context(), ue)
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

func checkUERegistrationDataIsCleared(ue *context.AmfUe) error {
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

	if ue.GetOnGoing() != context.OnGoingProcedureNothing {
		return fmt.Errorf("ongoing should be nothing")
	}

	return nil
}
