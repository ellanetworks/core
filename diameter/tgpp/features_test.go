// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"testing"

	"github.com/ellanetworks/core/diameter"
)

func TestSupportedFeaturesRoundTrip(t *testing.T) {
	f := SupportedFeatures{VendorID: VendorID, FeatureListID: 1, FeatureList: 0x5}

	a := f.AVP()
	if a.Code != AVPSupportedFeatures || a.VendorID != VendorID || a.Flags&diameter.AVPFlagMandatory != 0 {
		t.Fatalf("Supported-Features header = %+v", a)
	}

	got, err := ParseSupportedFeatures(a)
	if err != nil || got != f {
		t.Fatalf("ParseSupportedFeatures = %+v, %v", got, err)
	}
}

func TestParseSupportedFeaturesErrors(t *testing.T) {
	for name, a := range map[string]diameter.AVP{
		"not grouped": diameter.OctetString(AVPSupportedFeatures, 0, VendorID, []byte{0x01}),
		"no Feature-List": diameter.Grouped(AVPSupportedFeatures, 0, VendorID,
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, VendorID),
			diameter.Unsigned32(AVPFeatureListID, 0, VendorID, 1)),
		"short Feature-List-ID": diameter.Grouped(AVPSupportedFeatures, 0, VendorID,
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, VendorID),
			diameter.OctetString(AVPFeatureListID, 0, VendorID, []byte{0x01}),
			diameter.Unsigned32(AVPFeatureList, 0, VendorID, 1)),
	} {
		if _, err := ParseSupportedFeatures(a); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFeatureList(t *testing.T) {
	avps := []diameter.AVP{
		SupportedFeatures{VendorID: VendorID, FeatureListID: 1, FeatureList: 0x1}.AVP(),
		SupportedFeatures{VendorID: VendorID, FeatureListID: 1, FeatureList: 0x4}.AVP(),
		SupportedFeatures{VendorID: VendorID, FeatureListID: 2, FeatureList: 0x2}.AVP(),
		SupportedFeatures{VendorID: 9999, FeatureListID: 1, FeatureList: 0x8}.AVP(),
		diameter.OctetString(AVPSupportedFeatures, 0, VendorID, []byte{0x01}),
	}

	if got := FeatureList(avps, VendorID, 1); got != 0x5 {
		t.Fatalf("FeatureList = %#x, want 0x5", got)
	}

	if got := FeatureList(nil, VendorID, 1); got != 0 {
		t.Fatalf("FeatureList of nothing = %#x", got)
	}
}

func TestSupportedFeaturesMandatory(t *testing.T) {
	f := SupportedFeatures{VendorID: VendorID, FeatureListID: 2, FeatureList: 0x3, Mandatory: true}

	a := f.AVP()
	if a.Flags&diameter.AVPFlagMandatory == 0 {
		t.Fatalf("Supported-Features header = %+v", a)
	}

	got, err := ParseSupportedFeatures(a)
	if err != nil || got != f {
		t.Fatalf("ParseSupportedFeatures = %+v, %v", got, err)
	}
}

func TestFeaturesMandatory(t *testing.T) {
	optional := SupportedFeatures{VendorID: VendorID, FeatureListID: 1, FeatureList: 0x1}.AVP()
	mandatory := SupportedFeatures{VendorID: VendorID, FeatureListID: 2, FeatureList: 0x2, Mandatory: true}.AVP()
	otherVendor := SupportedFeatures{VendorID: 9999, FeatureListID: 1, FeatureList: 0x8, Mandatory: true}.AVP()

	for name, tc := range map[string]struct {
		avps []diameter.AVP
		want bool
	}{
		"none":                   {nil, false},
		"optional only":          {[]diameter.AVP{optional}, false},
		"one mandatory list":     {[]diameter.AVP{optional, mandatory}, true},
		"other vendor mandatory": {[]diameter.AVP{optional, otherVendor}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := FeaturesMandatory(tc.avps, VendorID); got != tc.want {
				t.Fatalf("FeaturesMandatory = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUnsupportedRequiredFeatures(t *testing.T) {
	supported := []SupportedFeatures{{VendorID: VendorID, FeatureListID: 1, FeatureList: 0x3}}
	required := func(vendorID, listID, list uint32) diameter.AVP {
		return SupportedFeatures{VendorID: vendorID, FeatureListID: listID, FeatureList: list, Mandatory: true}.AVP()
	}

	for name, tc := range map[string]struct {
		avps []diameter.AVP
		want bool
	}{
		"none":                      {nil, false},
		"supported required":        {[]diameter.AVP{required(VendorID, 1, 0x1)}, false},
		"unsupported bit":           {[]diameter.AVP{required(VendorID, 1, 0x4)}, true},
		"unknown list":              {[]diameter.AVP{required(VendorID, 2, 0x1)}, true},
		"other vendor":              {[]diameter.AVP{required(9999, 1, 0x1)}, true},
		"advertised only":           {[]diameter.AVP{SupportedFeatures{VendorID: VendorID, FeatureListID: 2, FeatureList: 0x1}.AVP()}, false},
		"malformed mandatory":       {[]diameter.AVP{diameter.Grouped(AVPSupportedFeatures, diameter.AVPFlagMandatory, VendorID)}, true},
		"malformed advertised only": {[]diameter.AVP{diameter.Grouped(AVPSupportedFeatures, 0, VendorID)}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := UnsupportedRequiredFeatures(tc.avps, supported...); got != tc.want {
				t.Fatalf("UnsupportedRequiredFeatures = %v, want %v", got, tc.want)
			}
		})
	}
}
