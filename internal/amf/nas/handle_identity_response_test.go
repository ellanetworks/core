// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

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
	ue           *amf.UeContext
	mi           []uint8
	expected_err error
	validate_ue  func(ue *amf.UeContext) error
}

func emptyValidation(ue *amf.UeContext) error {
	return nil
}

func newTestUe(integrityVerified bool, guti, oldGuti etsi.GUTI5G, tmsi etsi.TMSI) *amf.UeContext {
	_ = integrityVerified
	ue := amf.NewUeContext()
	ue.SetGutiForTest(guti)

	if tmsi != (etsi.TMSI{}) {
		ue.SetTmsiForTest(tmsi) // explicit 5G-S-TMSI cases
	}

	ue.SetOldTmsiForTest(oldGuti.Tmsi)

	return ue
}

func tmsiUe(integrityVerified bool, tmsi, oldTmsi etsi.TMSI) *amf.UeContext {
	_ = integrityVerified
	ue := amf.NewUeContext()
	ue.SetTmsiForTest(tmsi)
	ue.SetOldTmsiForTest(oldTmsi)

	return ue
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
			fmt.Errorf("amf.UeContext is nil"),
			emptyValidation,
		},
		{
			"Empty mobileIdentityContents",
			&amf.UeContext{},
			[]uint8{},
			fmt.Errorf("mobile identity is empty"),
			emptyValidation,
		},
		{
			"Unknown type is ignored",
			&amf.UeContext{},
			[]uint8{0xFF},
			nil,
			emptyValidation,
		},
		{
			"Invalid SUCI sets empty SUCI and PLMN",
			&amf.UeContext{},
			[]uint8{nasMessage.MobileIdentity5GSTypeSuci},
			nil,
			func(ue *amf.UeContext) error {
				if ue.Suci != "" || ue.PlmnID.Mcc != "" || ue.PlmnID.Mnc != "" {
					return fmt.Errorf("SUCI and PLMN should be empty, got %s, %s%s", ue.Suci, ue.PlmnID.Mcc, ue.PlmnID.Mnc)
				}

				return nil
			},
		},
		{
			"Valid SUCI sets SUCI and PLMN",
			&amf.UeContext{},
			[]uint8{nasMessage.MobileIdentity5GSTypeSuci, 0x00, 0xf1, 0x10, 0x10, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			nil,
			func(ue *amf.UeContext) error {
				if ue.Suci != "suci-0-001-01-0110-0-1-00000000000000000010" || ue.PlmnID.Mcc != "001" || ue.PlmnID.Mnc != "01" {
					return fmt.Errorf("SUCI and PLMN should not be empty, got %s, %s%s", ue.Suci, ue.PlmnID.Mcc, ue.PlmnID.Mnc)
				}

				return nil
			},
		},
		{
			"Invalid GUTI sets empty GUTI",
			newTestUe(false, mustTestGuti("999", "99", "cafe42", 0x00000001), etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0},
			fmt.Errorf("UE sent invalid GUTI: invalid GUTI length"),
			emptyValidation,
		},
		{
			"GUTI with MacFailed returns error",
			newTestUe(true, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0x10, 0x1f, 0, 0, 1, 0, 0, 0, 1},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid GUTI matches UE GUTI",
			newTestUe(false, mustTestGuti("001", "01", "cafe01", 0xdeadbeef), etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI matches UE old GUTI",
			newTestUe(false, mustTestGuti("001", "01", "cafe02", 0xf00df00d), mustTestGuti("001", "01", "cafe01", 0xdeadbeef), etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI does not match amf.AMF state",
			newTestUe(false, mustTestGuti("001", "01", "cafe02", 0xf00df00d), mustTestGuti("001", "01", "cafe01", 0x12345678), etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			fmt.Errorf("UE sent unknown GUTI"),
			emptyValidation,
		},
		{
			"5G-S-TMSI with MacFailed returns error",
			newTestUe(true, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0x00, 0x12, 0x34, 0x56, 0x78, 0x90},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"5G-S-TMSI maximum value matches",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, mustValidTestTmsi(0xFFFFFFFE)),
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE},
			nil,
			emptyValidation,
		},
		{
			"5G-S-TMSI too long returns error",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			fmt.Errorf("wrong length for TMSI"),
			emptyValidation,
		},
		{
			"5G-S-TMSI too short returns error",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0x01},
			fmt.Errorf("wrong length for TMSI"),
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE TMSI",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, mustValidTestTmsi(0x1A345678)),
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE old TMSI",
			tmsiUe(false, mustValidTestTmsi(0x22234567), mustValidTestTmsi(0x1A345678)),
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI does not match amf.AMF state",
			tmsiUe(false, mustValidTestTmsi(0x22234567), mustValidTestTmsi(0x5FFF5555)),
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			fmt.Errorf("UE sent unknown TMSI"),
			emptyValidation,
		},
		{
			"IMEI with MacFailed returns error",
			newTestUe(true, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSTypeImei + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEI sets PEI",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSTypeImei + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
			nil,
			func(ue *amf.UeContext) error {
				expected := "imei-490154203237518"
				if ue.Imei.String() != expected {
					return fmt.Errorf("PEI should be %s, got %s", expected, ue.Imei.String())
				}

				return nil
			},
		},
		{
			"IMEISV with MacFailed returns error",
			newTestUe(true, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEISV sets PEI",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
			nil,
			func(ue *amf.UeContext) error {
				expected := "imeisv-3520990017614823"
				if ue.Imei.String() != expected {
					return fmt.Errorf("PEI should be %s, got %s", expected, ue.Imei.String())
				}

				return nil
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			integrityVerified := !strings.Contains(tc.name, "MacFailed")
			err := updateUEIdentity(tc.ue, tc.mi, integrityVerified)

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
	testcases := []struct {
		name  string
		setup func(*amf.UeContext)
	}{
		{"Deregistered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Deregistered) }},
		{"Registered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Registered) }},
		{"SecurityMode", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepSecurityMode) }},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			tc.setup(ue)

			handleIdentityResponse(context.TODO(), amf.New(nil, nil, nil), ue, &nasMessage.IdentityResponse{}, true)

			if len(ngapSender.SentDownlinkNASTransport) != 0 {
				t.Fatalf("expected Identity Response in an invalid state to be ignored, but a downlink was sent")
			}
		})
	}
}

func TestHandleIdentityResponse_AuthenticationProcess_AuthenticationRequest(t *testing.T) {
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = ""
	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Tai = ue.Conn().Tai

	m := buildTestIdentityResponseMessage()

	handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse, true)

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
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = ""
	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Tai = models.Tai{}

	m := buildTestIdentityResponseMessage()

	handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse, true)

	if ue.State() != amf.Deregistered {
		t.Fatalf("expected UE to be deregistered after an authentication procedure failure, got %s", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleIdentityResponse_AuthenticationProcess_RegistrationAccept(t *testing.T) {
	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  supi,
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(supi)
	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

	registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCountForTest().Value())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.Conn().RegistrationRequest = registrationRequest.RegistrationRequest
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration

	m := buildTestIdentityResponseMessage()

	handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse, true)

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

	if err := security.NASEncrypt(ue.CipheringAlgForTest(), ue.KnasEncForTest(), ue.ULCountForTest().Value(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
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
			amfInstance := amf.New(&fakeDBInstance{
				Operator: &db.Operator{
					Mcc:           "001",
					Mnc:           "01",
					SupportedTACs: "[\"000001\"]",
				},
			}, &fakeAusf{
				AvKgAka: &ausf.AuthResult{
					Rand: hex.EncodeToString(make([]byte, 16)),
					Autn: hex.EncodeToString(make([]byte, 16)),
				},
				Supi:  supi,
				Kseaf: []byte("testkey"),
			}, nil)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.Suci = "testsuci"
			ue.SetSupiForTest(supi)
			ue.Imei, _ = etsi.NewIMEIFromPEI("imei-353456789012345")
			ue.ForceRegStepForTest(amf.RegStepContextSetup)
			ue.Tai = ue.Conn().Tai
			ue.SetSecuredForTest(true)
			{
				ng := ue.NgKsiForTest()
				ng.Ksi = 1
				ue.SetNgKsiForTest(ng)
			}

			key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
			algo := security.AlgCiphering128NEA2

			ue.SetKnasEncForTest(key)
			ue.SetKnasIntForTest(key)
			ue.SetCipheringAlgForTest(algo)
			ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

			registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCountForTest().Value())
			if err != nil {
				t.Fatalf("could not build registration request message: %v", err)
			}

			ue.Conn().RegistrationRequest = registrationRequest.RegistrationRequest

			ue.Conn().RegistrationType5GS = tc
			if tc == nasMessage.RegistrationType5GSMobilityRegistrationUpdating {
				ue.Conn().RegistrationRequest.Capability5GMM = &nasType.Capability5GMM{}
			}

			m := buildTestIdentityResponseMessage()

			handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse, true)

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

			if err := security.NASEncrypt(ue.CipheringAlgForTest(), ue.KnasEncForTest(), ue.ULCountForTest().Value(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
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
			amfInstance := amf.New(&fakeDBInstance{}, &fakeAusf{
				AvKgAka: &ausf.AuthResult{
					Rand: hex.EncodeToString(make([]byte, 16)),
					Autn: hex.EncodeToString(make([]byte, 16)),
				},
				Supi:  supi,
				Kseaf: []byte("testkey"),
			}, nil)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.Suci = "testsuci"
			ue.SetSupiForTest(supi)
			ue.Imei, _ = etsi.NewIMEIFromPEI("imei-353456789012345")
			ue.ForceRegStepForTest(amf.RegStepContextSetup)
			ue.Tai = ue.Conn().Tai
			ue.SetSecuredForTest(true)
			{
				ng := ue.NgKsiForTest()
				ng.Ksi = 1
				ue.SetNgKsiForTest(ng)
			}

			key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
			algo := security.AlgCiphering128NEA2

			ue.SetKnasEncForTest(key)
			ue.SetKnasIntForTest(key)
			ue.SetCipheringAlgForTest(algo)
			ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

			registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCountForTest().Value())
			if err != nil {
				t.Fatalf("could not build registration request message: %v", err)
			}

			ue.Conn().RegistrationRequest = registrationRequest.RegistrationRequest

			ue.Conn().RegistrationType5GS = tc
			if tc == nasMessage.RegistrationType5GSMobilityRegistrationUpdating {
				ue.Conn().RegistrationRequest.Capability5GMM = &nasType.Capability5GMM{}
			}

			m := buildTestIdentityResponseMessage()

			handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse, true)

			if len(ngapSender.SentDownlinkNASTransport) != 0 {
				t.Fatalf("should not have sent a Downlink NAS Transport message")
			}

			if len(ngapSender.SentUEContextReleaseCommand) != 1 {
				t.Fatalf("expected a UE Context Release Command to release the aborted registration, got %d", len(ngapSender.SentUEContextReleaseCommand))
			}
		})
	}
}

func TestHandleIdentityResponse_IdentityError(t *testing.T) {
	testcases := []struct {
		name  string
		setup func(*amf.UeContext)
	}{
		{"Authenticating", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepAuthenticating) }},
		{"ContextSetup", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepContextSetup) }},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			supi := mustSUPIFromPrefixed("imsi-001019756139935")
			amfInstance := amf.New(&fakeDBInstance{}, &fakeAusf{
				AvKgAka: &ausf.AuthResult{
					Rand: hex.EncodeToString(make([]byte, 16)),
					Autn: hex.EncodeToString(make([]byte, 16)),
				},
				Supi:  supi,
				Kseaf: []byte("testkey"),
			}, nil)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			tc.setup(ue)

			m := buildTestIdentityResponseMessage()
			m.SetMobileIdentityContents([]uint8{})
			m.IdentityResponse.SetLen(0)

			handleIdentityResponse(context.TODO(), amfInstance, ue, m.IdentityResponse, true)

			if len(ngapSender.SentDownlinkNASTransport) != 0 {
				t.Fatalf("should not have sent a Downlink NAS Transport message")
			}
		})
	}
}

// TestSendIdentityRequest_ArmsT3570 asserts SendIdentityRequest sends the
// IDENTITY REQUEST and arms T3570 to supervise the identification procedure
// (TS 24.501 §5.4.3.2), so a UE that never answers cannot leak its context.
func TestSendIdentityRequest_ArmsT3570(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	amf.SendIdentityRequest(context.TODO(), amf.New(nil, nil, nil), ue.Conn(), nasMessage.MobileIdentity5GSTypeSuci)

	if !ue.Conn().NASGuardForTest().Active() {
		t.Fatal("SendIdentityRequest must arm T3570")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected one IDENTITY REQUEST, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if err := nm.PlainNasDecode(&resp.NasPdu); err != nil {
		t.Fatalf("could not decode IDENTITY REQUEST: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeIdentityRequest {
		t.Fatalf("expected IDENTITY REQUEST, got %v", nm.GmmHeader.GetMessageType())
	}

	ue.Conn().NASGuardForTest().Stop()
}

// TestHandleIdentityResponse_T3570Stopped asserts the identification procedure
// is complete on receipt of the response: T3570 is stopped (TS 24.501 §5.4.3.4).
func TestHandleIdentityResponse_T3570Stopped(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	conn := ue.Conn()
	conn.NASGuardForTest().Arm(10*time.Minute, 5, func(int32) {}, func() {})

	handleIdentityResponse(context.TODO(), amf.New(nil, nil, nil), ue, &nasMessage.IdentityResponse{}, true)

	if conn.NASGuardForTest().Active() {
		t.Fatal("expected timer T3570 to be stopped on Identity Response")
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
