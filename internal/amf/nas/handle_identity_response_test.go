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
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
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
			[]uint8{uint8(fgs.IdentityNoIdentity)},
			fmt.Errorf("amf.UeContext is nil"),
			emptyValidation,
		},
		{
			"Empty mobileIdentityContents",
			&amf.UeContext{},
			[]uint8{},
			fmt.Errorf("nas/fgs: empty 5GS mobile identity"),
			emptyValidation,
		},
		{
			// An identity type this AMF does not model names no subscriber, so the
			// identification procedure cannot have succeeded (TS 24.501 §5.4.3.4).
			"Unknown type is refused",
			&amf.UeContext{},
			[]uint8{0xFF},
			fmt.Errorf("UE sent EUI-64"),
			emptyValidation,
		},
		{
			"Invalid SUCI sets empty SUCI and PLMN",
			&amf.UeContext{},
			[]uint8{uint8(fgs.IdentitySUCI)},
			fmt.Errorf("nas: bytes at octet 1: buffer truncated"),
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
			[]uint8{uint8(fgs.IdentitySUCI), 0x00, 0xf1, 0x10, 0x10, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
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
			[]uint8{uint8(fgs.IdentityGUTI), 0},
			fmt.Errorf("nas/fgs: 5G-GUTI is 2 octets, want 11"),
			emptyValidation,
		},
		{
			"GUTI with MacFailed returns error",
			newTestUe(true, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentityGUTI), 0, 0xf1, 0x10, 0, 0, 1, 0, 0, 0, 1},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid GUTI matches UE GUTI",
			newTestUe(false, mustTestGuti("001", "01", "cafe01", 0xdeadbeef), etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentityGUTI), 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI matches UE old GUTI",
			newTestUe(false, mustTestGuti("001", "01", "cafe02", 0xf00df00d), mustTestGuti("001", "01", "cafe01", 0xdeadbeef), etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentityGUTI), 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI does not match amf.AMF state",
			newTestUe(false, mustTestGuti("001", "01", "cafe02", 0xf00df00d), mustTestGuti("001", "01", "cafe01", 0x12345678), etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentityGUTI), 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			fmt.Errorf("UE sent unknown GUTI"),
			emptyValidation,
		},
		{
			"5G-S-TMSI with MacFailed returns error",
			newTestUe(true, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentitySTMSI), 0x00, 0x12, 0x34, 0x56, 0x78, 0x90},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"5G-S-TMSI maximum value matches",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, mustValidTestTmsi(0xFFFFFFFE)),
			[]uint8{uint8(fgs.IdentitySTMSI), 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE},
			nil,
			emptyValidation,
		},
		{
			"5G-S-TMSI too long returns error",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentitySTMSI), 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			fmt.Errorf("nas/fgs: 5G-S-TMSI: want 7 octets, got 8"),
			emptyValidation,
		},
		{
			"5G-S-TMSI too short returns error",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentitySTMSI), 0xFF, 0xFF, 0x01},
			fmt.Errorf("nas/fgs: 5G-S-TMSI: want 7 octets, got 4"),
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE TMSI",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, mustValidTestTmsi(0x1A345678)),
			[]uint8{uint8(fgs.IdentitySTMSI), 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE old TMSI",
			tmsiUe(false, mustValidTestTmsi(0x22234567), mustValidTestTmsi(0x1A345678)),
			[]uint8{uint8(fgs.IdentitySTMSI), 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI does not match amf.AMF state",
			tmsiUe(false, mustValidTestTmsi(0x22234567), mustValidTestTmsi(0x5FFF5555)),
			[]uint8{uint8(fgs.IdentitySTMSI), 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			fmt.Errorf("UE sent unknown TMSI"),
			emptyValidation,
		},
		{
			"IMEI with MacFailed returns error",
			newTestUe(true, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentityIMEI) + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEI sets PEI",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentityIMEI) + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
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
			[]uint8{uint8(fgs.IdentityIMEISV) + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEISV sets PEI",
			newTestUe(false, etsi.GUTI5G{}, etsi.GUTI5G{}, etsi.TMSI{}),
			[]uint8{uint8(fgs.IdentityIMEISV) + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
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

			// A malformed identity is rejected by the decoder, before the handler
			// ever sees it; the outcome under test is the same either way.
			id, err := fgs.ParseMobileIdentity(tc.mi)
			if err == nil {
				err = updateUEIdentity(tc.ue, id, integrityVerified)
			}

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

			handleIdentityResponse(context.TODO(), amf.New(nil, nil, nil), ue, buildTestIdentityResponse(t), true)

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

	m := buildTestIdentityResponse(t)

	handleIdentityResponse(context.TODO(), amfInstance, ue, m, true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgAuthenticationRequest))
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

	m := buildTestIdentityResponse(t)

	handleIdentityResponse(context.TODO(), amfInstance, ue, m, true)

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
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	ue.Conn().RegistrationRequest = &fgs.RegistrationRequest{}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeInitial

	m := buildTestIdentityResponse(t)

	handleIdentityResponse(context.TODO(), amfInstance, ue, m, true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmm(t, ue, resp.NASPDU, uint8(fgs.MsgRegistrationAccept))
}

func TestHandleIdentityResponse_ContextSetup_RegistrationAccept(t *testing.T) {
	testcases := []uint8{
		uint8(fgs.RegistrationTypeInitial),
		uint8(fgs.RegistrationTypeMobilityUpdating),
		uint8(fgs.RegistrationTypePeriodicUpdating),
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
			algo := nas.CipheringAES

			ue.SetKnasEncForTest(key)
			ue.SetKnasIntForTest(key)
			ue.SetCipheringAlgForTest(algo)
			ue.SetIntegrityAlgForTest(nas.IntegrityNull)

			ue.Conn().RegistrationRequest = &fgs.RegistrationRequest{}

			ue.Conn().RegistrationType5GS = fgs.RegistrationType(tc)
			if fgs.RegistrationType(tc) == fgs.RegistrationTypeMobilityUpdating {
				ue.Conn().RegistrationRequest.GMMCapability = &fgs.GMMCapability{}
			}

			m := buildTestIdentityResponse(t)

			handleIdentityResponse(context.TODO(), amfInstance, ue, m, true)

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("should have sent a Downlink NAS Transport message")
			}

			resp := ngapSender.SentDownlinkNASTransport[0]
			decipherGmm(t, ue, resp.NASPDU, uint8(fgs.MsgRegistrationAccept))
		})
	}
}

func TestHandleIdentityResponse_ContextSetup_Error(t *testing.T) {
	testcases := []uint8{
		uint8(fgs.RegistrationTypeInitial),
		uint8(fgs.RegistrationTypeMobilityUpdating),
		uint8(fgs.RegistrationTypePeriodicUpdating),
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
			algo := nas.CipheringAES

			ue.SetKnasEncForTest(key)
			ue.SetKnasIntForTest(key)
			ue.SetCipheringAlgForTest(algo)
			ue.SetIntegrityAlgForTest(nas.IntegrityNull)

			ue.Conn().RegistrationRequest = &fgs.RegistrationRequest{}

			ue.Conn().RegistrationType5GS = fgs.RegistrationType(tc)
			if fgs.RegistrationType(tc) == fgs.RegistrationTypeMobilityUpdating {
				ue.Conn().RegistrationRequest.GMMCapability = &fgs.GMMCapability{}
			}

			m := buildTestIdentityResponse(t)

			handleIdentityResponse(context.TODO(), amfInstance, ue, m, true)

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

			m := buildTestIdentityResponseEmpty()

			handleIdentityResponse(context.TODO(), amfInstance, ue, m, true)

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

	amf.SendIdentityRequest(context.TODO(), amf.New(nil, nil, nil), ue.Conn(), fgs.IdentitySUCI)

	if !ue.Conn().NASGuardForTest().Active() {
		t.Fatal("SendIdentityRequest must arm T3570")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected one IDENTITY REQUEST, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgIdentityRequest))

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

	handleIdentityResponse(context.TODO(), amf.New(nil, nil, nil), ue, buildTestIdentityResponse(t), true)

	if conn.NASGuardForTest().Active() {
		t.Fatal("expected timer T3570 to be stopped on Identity Response")
	}
}

// buildTestIdentityResponseMessage builds a plain IDENTITY RESPONSE carrying a
// SUCI mobile identity (type-6 IE, 2-octet LVE length).
func buildTestIdentityResponseMessage() []byte {
	return buildIdentityResponsePlain([]byte{0x01, 0x00, 0xf1, 0x10, 0x10, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
}

// buildTestIdentityResponse is the same message, decoded.
func buildTestIdentityResponse(t *testing.T) *fgs.IdentityResponse {
	t.Helper()

	resp, err := fgs.ParseIdentityResponse(buildTestIdentityResponseMessage())
	if err != nil {
		t.Fatalf("build IDENTITY RESPONSE: %v", err)
	}

	return resp
}

// buildTestIdentityResponseEmpty is an IDENTITY RESPONSE carrying no identity.
func buildTestIdentityResponseEmpty() *fgs.IdentityResponse {
	return &fgs.IdentityResponse{}
}

func buildIdentityResponsePlain(mobileIdentity []byte) []byte {
	b := []byte{uint8(fgs.EPD5GMM), 0x00, uint8(fgs.MsgIdentityResponse)}
	b = append(b, byte(len(mobileIdentity)>>8), byte(len(mobileIdentity)))

	return append(b, mobileIdentity...)
}
