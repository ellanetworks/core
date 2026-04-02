package gmm

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

type UpdateInputs struct {
	name         string
	ue           *amf.AmfUe
	mi           []uint8
	expected_err error
	validate_ue  func(ue *amf.AmfUe) error
}

func emptyValidation(ue *amf.AmfUe) error {
	return nil
}

func mustValidTestTmsi(t uint32) etsi.TMSI {
	tmsi, err := etsi.NewTMSI(t)
	if err != nil {
		panic("Tried to create an invalid test TMSI")
	}

	return tmsi
}

func TestUpdateUeIdentity(t *testing.T) {
	testcases := []UpdateInputs{
		{
			"NIL UE",
			nil,
			[]uint8{},
			fmt.Errorf("AmfUe is nil"),
			emptyValidation,
		},
		{
			"Empty mobileIdentityContents",
			&amf.AmfUe{},
			[]uint8{},
			fmt.Errorf("mobile identity is empty"),
			emptyValidation,
		},
		{
			"Unknown type is ignored",
			&amf.AmfUe{},
			[]uint8{0xFF},
			nil,
			emptyValidation,
		},
		{
			"Invalid SUCI sets empty SUCI and PLMN",
			&amf.AmfUe{},
			[]uint8{nasMessage.MobileIdentity5GSTypeSuci},
			nil,
			func(ue *amf.AmfUe) error {
				if ue.Suci != "" || ue.PlmnID.Mcc != "" || ue.PlmnID.Mnc != "" {
					return fmt.Errorf("SUCI and PLMN should be empty, got %s, %s%s", ue.Suci, ue.PlmnID.Mcc, ue.PlmnID.Mnc)
				}

				return nil
			},
		},
		{
			"Valid SUCI sets SUCI and PLMN",
			&amf.AmfUe{},
			[]uint8{nasMessage.MobileIdentity5GSTypeSuci, 0x00, 0xf1, 0x10, 0x10, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			nil,
			func(ue *amf.AmfUe) error {
				if ue.Suci != "suci-0-001-01-0110-0-1-00000000000000000010" || ue.PlmnID.Mcc != "001" || ue.PlmnID.Mnc != "01" {
					return fmt.Errorf("SUCI and PLMN should not be empty, got %s, %s%s", ue.Suci, ue.PlmnID.Mcc, ue.PlmnID.Mnc)
				}

				return nil
			},
		},
		{
			"Invalid GUTI sets empty GUTI",
			&amf.AmfUe{Guti: mustTestGuti("999", "99", "cafe42", 0x00000001), MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0},
			fmt.Errorf("UE sent invalid GUTI: invalid GUTI length"),
			emptyValidation,
		},
		{
			"GUTI with MacFailed returns error",
			&amf.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0x10, 0x1f, 0, 0, 1, 0, 0, 0, 1},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid GUTI matches UE GUTI",
			&amf.AmfUe{MacFailed: false, Guti: mustTestGuti("001", "01", "cafe01", 0xdeadbeef)},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI matches UE old GUTI",
			&amf.AmfUe{MacFailed: false, Guti: mustTestGuti("001", "01", "cafe02", 0xf00df00d), OldGuti: mustTestGuti("001", "01", "cafe01", 0xdeadbeef)},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI does not match AMF state",
			&amf.AmfUe{MacFailed: false, Guti: mustTestGuti("001", "01", "cafe02", 0xf00df00d), OldGuti: mustTestGuti("001", "01", "cafe01", 0x12345678)},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			fmt.Errorf("UE sent unknown GUTI"),
			emptyValidation,
		},
		{
			"5G-S-TMSI with MacFailed returns error",
			&amf.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0x00, 0x12, 0x34, 0x56, 0x78, 0x90},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"5G-S-TMSI maximum value matches",
			&amf.AmfUe{MacFailed: false, Tmsi: mustValidTestTmsi(0xFFFFFFFE)},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE},
			nil,
			emptyValidation,
		},
		{
			"5G-S-TMSI too long returns error",
			&amf.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			fmt.Errorf("wrong length for TMSI"),
			emptyValidation,
		},
		{
			"5G-S-TMSI too short returns error",
			&amf.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0x01},
			fmt.Errorf("wrong length for TMSI"),
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE TMSI",
			&amf.AmfUe{MacFailed: false, Tmsi: mustValidTestTmsi(0x1A345678)},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE old TMSI",
			&amf.AmfUe{MacFailed: false, Tmsi: mustValidTestTmsi(0x22234567), OldTmsi: mustValidTestTmsi(0x1A345678)},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI does not match AMF state",
			&amf.AmfUe{MacFailed: false, Tmsi: mustValidTestTmsi(0x22234567), OldTmsi: mustValidTestTmsi(0x5FFF5555)},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			fmt.Errorf("UE sent unknown TMSI"),
			emptyValidation,
		},
		{
			"IMEI with MacFailed returns error",
			&amf.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSTypeImei + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEI sets PEI",
			&amf.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSTypeImei + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
			nil,
			func(ue *amf.AmfUe) error {
				expected := "imei-490154203237518"
				if ue.Pei != expected {
					return fmt.Errorf("PEI should be %s, got %s", expected, ue.Pei)
				}

				return nil
			},
		},
		{
			"IMEISV with MacFailed returns error",
			&amf.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEISV sets PEI",
			&amf.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
			nil,
			func(ue *amf.AmfUe) error {
				expected := "imeisv-3520990017614823"
				if ue.Pei != expected {
					return fmt.Errorf("PEI should be %s, got %s", expected, ue.Pei)
				}

				return nil
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := updateUEIdentity(tc.ue, tc.mi)

			if tc.expected_err == nil && err != nil {
				t.Fatalf("expected error to be nil, got %v", err)
			} else if tc.expected_err != nil && err == nil {
				t.Fatalf("expected an error but error was nil")
			} else if tc.expected_err != nil && err != nil && err.Error() != tc.expected_err.Error() {
				t.Fatalf("expected error to be %v, got %v", tc.expected_err, err)
			}

			if err = tc.validate_ue(tc.ue); err != nil {
				t.Fatalf("validating updated UE failed: %v", err)
			}
		})
	}
}

func TestHandleIdentityResponse_InvalidStateError(t *testing.T) {
	testcases := []amf.StateType{amf.Deregistered, amf.Registered, amf.SecurityMode}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := amf.NewAmfUe()
			ue.ForceState(tc)

			err := handleIdentityResponse(context.TODO(), amf.New(nil, nil, nil), ue, &nasMessage.IdentityResponse{})
			if err == nil {
				t.Fatalf("expected an state mismatch error, got no error")
			}
		})
	}
}

func TestHandleIdentityResponse_AuthenticationProcess_AuthenticationRequest(t *testing.T) {
	amfInstance := amf.New(&FakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &FakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: "testkey",
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = ""
	ue.ForceState(amf.Authentication)
	ue.MacFailed = false
	ue.Tai = ue.RanUe().Tai

	m := buildTestIdentityResponseMessage()

	err = handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse)
	if err != nil {
		t.Fatalf("expected no errors but got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationRequest {
		t.Fatalf("expected an authentication request message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleIdentityResponse_AuthenticationProcess_AuthenticationError(t *testing.T) {
	amfInstance := amf.New(&FakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &FakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: "testkey",
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = ""
	ue.ForceState(amf.Authentication)
	ue.MacFailed = false
	ue.Tai = models.Tai{}

	m := buildTestIdentityResponseMessage()

	expected := "error in authentication procedure: failed to send ue authentication request: tai is not available in UE context"

	err = handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse)
	if err == nil {
		t.Fatalf("expected error but got none")
	}

	if err.Error() != expected {
		t.Fatalf("expected error: %v, got %v", expected, err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleIdentityResponse_AuthenticationProcess_RegistrationAccept(t *testing.T) {
	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	amfInstance := amf.New(&FakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &FakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  supi,
		Kseaf: "testkey",
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = supi
	ue.ForceState(amf.Authentication)
	ue.MacFailed = false
	ue.Tai = ue.RanUe().Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount.Get())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.RegistrationRequest = registrationRequest.RegistrationRequest
	ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration

	m := buildTestIdentityResponseMessage()

	err = handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse)
	if err != nil {
		t.Fatalf("expected no errors but got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload := make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a protected and ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected a registration accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleIdentityResponse_ContextSetup_RegistrationAccept(t *testing.T) {
	testcases := []uint8{
		nasMessage.RegistrationType5GSInitialRegistration,
		nasMessage.RegistrationType5GSMobilityRegistrationUpdating,
		nasMessage.RegistrationType5GSPeriodicRegistrationUpdating,
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			supi := mustSUPIFromPrefixed("imsi-001019756139935")
			amfInstance := amf.New(&FakeDBInstance{
				Operator: &db.Operator{
					Mcc:           "001",
					Mnc:           "01",
					SupportedTACs: "[\"000001\"]",
				},
			}, &FakeAusf{
				AvKgAka: &ausf.AuthResult{
					Rand: hex.EncodeToString(make([]byte, 16)),
					Autn: hex.EncodeToString(make([]byte, 16)),
				},
				Supi:  supi,
				Kseaf: "testkey",
			}, nil)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.Suci = "testsuci"
			ue.Supi = supi
			ue.Pei = "testpei"
			ue.ForceState(amf.ContextSetup)
			ue.MacFailed = false
			ue.Tai = ue.RanUe().Tai
			ue.SecurityContextAvailable = true
			ue.NgKsi.Ksi = 1
			key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
			algo := security.AlgCiphering128NEA2
			ue.KnasEnc = key
			ue.KnasInt = key
			ue.CipheringAlg = algo
			ue.IntegrityAlg = security.AlgIntegrity128NIA0

			registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount.Get())
			if err != nil {
				t.Fatalf("could not build registration request message: %v", err)
			}

			ue.RegistrationRequest = registrationRequest.RegistrationRequest

			ue.RegistrationType5GS = tc
			if tc == nasMessage.RegistrationType5GSMobilityRegistrationUpdating {
				ue.RegistrationRequest.Capability5GMM = &nasType.Capability5GMM{}
			}

			m := buildTestIdentityResponseMessage()

			err = handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse)
			if err != nil {
				t.Fatalf("expected no errors but got: %v", err)
			}

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("should have sent a Downlink NAS Transport message")
			}

			resp := ngapSender.SentDownlinkNASTransport[0]
			nm := new(nas.Message)
			nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

			payload := make([]byte, len(resp.NasPdu))
			copy(payload, resp.NasPdu)
			payload = payload[7:]

			if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
				t.Fatalf("expected a protected and ciphered NAS message")
			}

			if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
				t.Fatalf("could not decrypt NAS message: %v", err)
			}

			err = nm.PlainNasDecode(&payload)
			if err != nil {
				t.Fatalf("could not decode ciphered NAS message: %v", err)
			}

			if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
				t.Fatalf("expected a registration accept message, got '%v'", nm.GmmHeader.GetMessageType())
			}
		})
	}
}

func TestHandleIdentityResponse_ContextSetup_Error(t *testing.T) {
	testcases := []uint8{
		nasMessage.RegistrationType5GSInitialRegistration,
		nasMessage.RegistrationType5GSMobilityRegistrationUpdating,
		nasMessage.RegistrationType5GSPeriodicRegistrationUpdating,
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			supi := mustSUPIFromPrefixed("imsi-001019756139935")
			amfInstance := amf.New(&FakeDBInstance{}, &FakeAusf{
				AvKgAka: &ausf.AuthResult{
					Rand: hex.EncodeToString(make([]byte, 16)),
					Autn: hex.EncodeToString(make([]byte, 16)),
				},
				Supi:  supi,
				Kseaf: "testkey",
			}, nil)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.Suci = "testsuci"
			ue.Supi = supi
			ue.Pei = "testpei"
			ue.ForceState(amf.ContextSetup)
			ue.MacFailed = false
			ue.Tai = ue.RanUe().Tai
			ue.SecurityContextAvailable = true
			ue.NgKsi.Ksi = 1
			key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
			algo := security.AlgCiphering128NEA2
			ue.KnasEnc = key
			ue.KnasInt = key
			ue.CipheringAlg = algo
			ue.IntegrityAlg = security.AlgIntegrity128NIA0

			registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount.Get())
			if err != nil {
				t.Fatalf("could not build registration request message: %v", err)
			}

			ue.RegistrationRequest = registrationRequest.RegistrationRequest

			ue.RegistrationType5GS = tc
			if tc == nasMessage.RegistrationType5GSMobilityRegistrationUpdating {
				ue.RegistrationRequest.Capability5GMM = &nasType.Capability5GMM{}
			}

			m := buildTestIdentityResponseMessage()

			err = handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse)
			if err == nil {
				t.Fatalf("expected error but got none")
			}

			if len(ngapSender.SentDownlinkNASTransport) != 0 {
				t.Fatalf("should not have sent a Downlink NAS Transport message")
			}

			if ue.GetState() != amf.Deregistered {
				t.Fatalf("ue should have transitioned to Deregistered state, but got: %v", ue.GetState())
			}
		})
	}
}

func TestHandleIdentityResponse_IdentityError(t *testing.T) {
	testcases := []amf.StateType{amf.Authentication, amf.ContextSetup}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			supi := mustSUPIFromPrefixed("imsi-001019756139935")
			amfInstance := amf.New(&FakeDBInstance{}, &FakeAusf{
				AvKgAka: &ausf.AuthResult{
					Rand: hex.EncodeToString(make([]byte, 16)),
					Autn: hex.EncodeToString(make([]byte, 16)),
				},
				Supi:  supi,
				Kseaf: "testkey",
			}, nil)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.ForceState(tc)

			m := buildTestIdentityResponseMessage()
			m.SetMobileIdentityContents([]uint8{})
			m.IdentityResponse.SetLen(0)

			expected := "error handling identity response: mobile identity is empty"

			err = handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse)
			if err == nil {
				t.Fatalf("expected error but got none")
			}

			if err.Error() != expected {
				t.Fatalf("expected error: %v, got %v", expected, err)
			}

			if len(ngapSender.SentDownlinkNASTransport) != 0 {
				t.Fatalf("should not have sent a Downlink NAS Transport message")
			}
		})
	}
}

func buildTestIdentityResponseMessage() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	identityResponse := nasMessage.NewIdentityResponse(0)
	identityResponse.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	identityResponse.SetSpareHalfOctet(0x00)
	identityResponse.SetMessageType(nas.MsgTypeIdentityResponse)
	identityResponse.SetLen(18)
	identityResponse.SetMobileIdentityContents([]uint8{nasMessage.MobileIdentity5GSTypeSuci, 0x00, 0xf1, 0x10, 0x10, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})

	m.IdentityResponse = identityResponse
	m.SetMessageType(nas.MsgTypeIdentityResponse)

	return m
}
