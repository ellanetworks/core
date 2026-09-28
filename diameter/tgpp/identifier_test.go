// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
)

func TestUserIdentifierRoundTrip(t *testing.T) {
	for name, u := range map[string]UserIdentifier{
		"IMSI and MSISDN": {IMSI: "001010000000001", MSISDN: "15551230002"},
		"IMSI only":       {IMSI: "001010000000001"},
		"MSISDN only":     {MSISDN: "15551230002"},
	} {
		t.Run(name, func(t *testing.T) {
			a, err := NewUserIdentifier(u)
			if err != nil {
				t.Fatal(err)
			}

			if a.Code != AVPUserIdentifier || a.VendorID != VendorID || a.Flags&diameter.AVPFlagMandatory == 0 {
				t.Fatalf("User-Identifier header = %+v", a)
			}

			got, err := ParseUserIdentifier(a)
			if err != nil || got != u {
				t.Fatalf("ParseUserIdentifier = %+v, %v", got, err)
			}
		})
	}
}

func TestUserIdentifierEncoding(t *testing.T) {
	a, err := NewUserIdentifier(UserIdentifier{IMSI: "001010000000001", MSISDN: "15551230002"})
	if err != nil {
		t.Fatal(err)
	}

	inner, err := a.Grouped()
	if err != nil {
		t.Fatal(err)
	}

	userName, _ := diameter.Find(inner, diameter.AVPUserName, 0)
	msisdn, _ := diameter.Find(inner, AVPMSISDN, VendorID)

	if userName.UTF8String() != "001010000000001" || userName.Flags&diameter.AVPFlagMandatory == 0 {
		t.Fatalf("User-Name = %+v", userName)
	}

	if !bytes.Equal(msisdn.Data, []byte{0x51, 0x55, 0x21, 0x03, 0x00, 0xf2}) || msisdn.Flags&diameter.AVPFlagMandatory == 0 {
		t.Fatalf("MSISDN = %+v", msisdn)
	}
}

func TestNewUserIdentifierErrors(t *testing.T) {
	for name, u := range map[string]UserIdentifier{
		"empty":      {},
		"bad MSISDN": {MSISDN: "+1555"},
		"bad IMSI":   {IMSI: "12a"},
	} {
		if _, err := NewUserIdentifier(u); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseUserIdentifierErrors(t *testing.T) {
	for name, a := range map[string]diameter.AVP{
		"not grouped": diameter.OctetString(AVPUserIdentifier, diameter.AVPFlagMandatory, VendorID, []byte{0x01}),
		"empty":       diameter.Grouped(AVPUserIdentifier, diameter.AVPFlagMandatory, VendorID),
		"external identifier only": diameter.Grouped(AVPUserIdentifier, diameter.AVPFlagMandatory, VendorID,
			diameter.UTF8String(3111, 0, VendorID, "device@example.org")),
		"bad MSISDN": diameter.Grouped(AVPUserIdentifier, diameter.AVPFlagMandatory, VendorID,
			diameter.OctetString(AVPMSISDN, diameter.AVPFlagMandatory, VendorID, []byte{0xba})),
		"bad IMSI": diameter.Grouped(AVPUserIdentifier, diameter.AVPFlagMandatory, VendorID,
			diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "hss-id.example.org")),
	} {
		if _, err := ParseUserIdentifier(a); !errors.Is(err, ErrInvalidUserIdentifier) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
