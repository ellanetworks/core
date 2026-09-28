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
