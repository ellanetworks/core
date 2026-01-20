package gmm

import (
	"fmt"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas/nasMessage"
)

type UpdateInputs struct {
	name         string
	ue           *amfContext.AmfUe
	mi           []uint8
	expected_err error
	validate_ue  func(ue *amfContext.AmfUe) error
}

func emptyValidation(ue *amfContext.AmfUe) error {
	return nil
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
			&amfContext.AmfUe{},
			[]uint8{},
			fmt.Errorf("mobile identity is empty"),
			emptyValidation,
		},
		{
			"Unknown type is ignored",
			&amfContext.AmfUe{},
			[]uint8{0xFF},
			nil,
			emptyValidation,
		},
		{
			"Invalid SUCI sets empty SUCI and PLMN",
			&amfContext.AmfUe{},
			[]uint8{nasMessage.MobileIdentity5GSTypeSuci},
			nil,
			func(ue *amfContext.AmfUe) error {
				if ue.Suci != "" || ue.PlmnID.Mcc != "" || ue.PlmnID.Mnc != "" {
					return fmt.Errorf("SUCI and PLMN should be empty, got %s, %s%s", ue.Suci, ue.PlmnID.Mcc, ue.PlmnID.Mnc)
				}

				return nil
			},
		},
		{
			"Valid SUCI sets SUCI and PLMN",
			&amfContext.AmfUe{},
			[]uint8{nasMessage.MobileIdentity5GSTypeSuci, 0x00, 0xf1, 0x10, 0x10, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			nil,
			func(ue *amfContext.AmfUe) error {
				if ue.Suci != "suci-0-001-01-0110-0-1-00000000000000000010" || ue.PlmnID.Mcc != "001" || ue.PlmnID.Mnc != "01" {
					return fmt.Errorf("SUCI and PLMN should not be empty, got %s, %s%s", ue.Suci, ue.PlmnID.Mcc, ue.PlmnID.Mnc)
				}

				return nil
			},
		},
		{
			"Invalid GUTI sets empty GUTI",
			&amfContext.AmfUe{Guti: "oldguti", MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0},
			fmt.Errorf("UE sent invalid GUTI"),
			emptyValidation,
		},
		{
			"GUTI with MacFailed returns error",
			&amfContext.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0x10, 0x1f, 0, 0, 1, 0, 0, 0, 1},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid GUTI matches UE GUTI",
			&amfContext.AmfUe{MacFailed: false, Guti: "00101cafe01deadbeef"},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI matches UE old GUTI",
			&amfContext.AmfUe{MacFailed: false, Guti: "00101cafe02f00df00d", OldGuti: "00101cafe01deadbeef"},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			nil,
			emptyValidation,
		},
		{
			"Valid GUTI does not match AMF state",
			&amfContext.AmfUe{MacFailed: false, Guti: "00101cafe02f00df00d", OldGuti: "00101cafe0112345678"},
			[]uint8{nasMessage.MobileIdentity5GSType5gGuti, 0, 0xf1, 0x10, 0xCA, 0xFE, 1, 0xDE, 0xAD, 0xBE, 0xEF},
			fmt.Errorf("UE sent unknown GUTI"),
			emptyValidation,
		},
		{
			"5G-S-TMSI with MacFailed returns error",
			&amfContext.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0x00, 0x12, 0x34, 0x56, 0x78, 0x90},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"5G-S-TMSI maximum value matches",
			&amfContext.AmfUe{MacFailed: false, Tmsi: 0xFFFFFFFF},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			nil,
			emptyValidation,
		},
		{
			"5G-S-TMSI too long returns error",
			&amfContext.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			fmt.Errorf("wrong length for TMSI"),
			emptyValidation,
		},
		{
			"5G-S-TMSI too short returns error",
			&amfContext.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFF, 0xFF, 0x01},
			fmt.Errorf("wrong length for TMSI"),
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE TMSI",
			&amfContext.AmfUe{MacFailed: false, Tmsi: uint32(0x1A345678)},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI matches UE old TMSI",
			&amfContext.AmfUe{MacFailed: false, Tmsi: uint32(0x22234567), OldTmsi: uint32(0x1A345678)},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			nil,
			emptyValidation,
		},
		{
			"Valid 5G-S-TMSI does not match AMF state",
			&amfContext.AmfUe{MacFailed: false, Tmsi: uint32(0x22234567), OldTmsi: uint32(0x5FFF5555)},
			[]uint8{nasMessage.MobileIdentity5GSType5gSTmsi, 0xFE, 0x01, 0x1A, 0x34, 0x56, 0x78},
			fmt.Errorf("UE sent unknown TMSI"),
			emptyValidation,
		},
		{
			"IMEI with MacFailed returns error",
			&amfContext.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSTypeImei + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEI sets PEI",
			&amfContext.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSTypeImei + 0x08 + 0x40, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
			nil,
			func(ue *amfContext.AmfUe) error {
				expected := "imei-490154203237518"
				if ue.Pei != expected {
					return fmt.Errorf("PEI should be %s, got %s", expected, ue.Pei)
				}

				return nil
			},
		},
		{
			"IMEISV with MacFailed returns error",
			&amfContext.AmfUe{MacFailed: true},
			[]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
			fmt.Errorf("NAS message integrity check failed"),
			emptyValidation,
		},
		{
			"Valid IMEISV sets PEI",
			&amfContext.AmfUe{MacFailed: false},
			[]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
			nil,
			func(ue *amfContext.AmfUe) error {
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
